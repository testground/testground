# EKS Worker Nodes Resources
#  * IAM role allowing Kubernetes actions to access other AWS services
#  * EC2 Security Group to allow networking traffic
#  * Data source to fetch latest EKS worker AMI
#  * AutoScaling Launch Configuration to configure worker instances
#  * AutoScaling Group to launch worker instances

resource "aws_iam_role" "testground-node" {
  name = "${var.cluster-name}-node"

  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
POLICY

}

resource "aws_iam_role_policy_attachment" "testground-node-AmazonEKSWorkerNodePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.testground-node.name
}

resource "aws_iam_role_policy_attachment" "testground-node-AmazonEKS_CNI_Policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.testground-node.name
}

resource "aws_iam_role_policy_attachment" "testground-node-AmazonEC2ContainerRegistryReadOnly" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.testground-node.name
}

resource "aws_iam_instance_profile" "testground-node" {
  name = var.cluster-name
  role = aws_iam_role.testground-node.name
}

resource "aws_security_group" "testground-node" {
  name        = "${var.cluster-name}-node"
  description = "Security group for all nodes in the cluster"
  vpc_id      = aws_vpc.testground.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    "Name"                                      = "${var.cluster-name}-node"
    "kubernetes.io/cluster/${var.cluster-name}" = "owned"
  }
}

resource "aws_security_group_rule" "testground-node-ingress-self" {
  description              = "Allow node to communicate with each other"
  from_port                = 0
  protocol                 = "-1"
  security_group_id        = aws_security_group.testground-node.id
  source_security_group_id = aws_security_group.testground-node.id
  to_port                  = 65535
  type                     = "ingress"
}

resource "aws_security_group_rule" "testground-node-ingress-cluster" {
  description              = "Allow worker Kubelets and pods to receive communication from the cluster control plane"
  from_port                = 1025
  protocol                 = "tcp"
  security_group_id        = aws_security_group.testground-node.id
  source_security_group_id = aws_security_group.testground-cluster.id
  to_port                  = 65535
  type                     = "ingress"
}

resource "aws_security_group_rule" "testground-node-ingress-services" {
  description       = "Allow worker Kubelets and pods to receive communication from the Internet to ports 30000-32767"
  from_port         = 30000
  to_port           = 32767
  protocol          = "tcp"
  security_group_id = aws_security_group.testground-node.id
  type              = "ingress"

  cidr_blocks = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "testground-node-ssh" {
  description       = "Allow SSH on worker Kubelets"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  security_group_id = aws_security_group.testground-node.id
  type              = "ingress"

  cidr_blocks = ["0.0.0.0/0"]
}

data "aws_ami" "eks_node_ami" {
  filter {
    name = "name"
    values = ["amazon-eks-node-1.14-v20190927"]
  }

  most_recent = true
  owners = ["602401143452"] # Amazon EKS AMI Account ID
}

# EKS currently documents this required userdata for EKS worker nodes to
# properly configure Kubernetes applications on the EC2 instance.
# We utilize a Terraform local here to simplify Base64 encoding this
# information into the AutoScaling Launch Configuration.
# More information: https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-06-05/amazon-eks-nodegroup.yaml
locals {
  testground-node-userdata = <<USERDATA
#!/bin/bash -xe

echo '{ "log-driver": "json-file", "log-opts": { "max-size": "20m", "max-file": "10" }}' > /etc/docker/daemon.json
systemctl restart docker

CA_CERTIFICATE_DIRECTORY=/etc/kubernetes/pki
CA_CERTIFICATE_FILE_PATH=$CA_CERTIFICATE_DIRECTORY/ca.crt
mkdir -p $CA_CERTIFICATE_DIRECTORY
echo "${aws_eks_cluster.testground.certificate_authority[0].data}" | base64 -d >  $CA_CERTIFICATE_FILE_PATH
INTERNAL_IP=$(curl -s http://169.254.169.254/latest/meta-data/local-ipv4)
sed -i s,MASTER_ENDPOINT,${aws_eks_cluster.testground.endpoint},g /var/lib/kubelet/kubeconfig
sed -i s,CLUSTER_NAME,${var.cluster-name},g /var/lib/kubelet/kubeconfig
sed -i s,REGION,${data.aws_region.current.name},g /etc/systemd/system/kubelet.service
sed -i s,MAX_PODS,300,g /etc/systemd/system/kubelet.service
sed -i s,MASTER_ENDPOINT,${aws_eks_cluster.testground.endpoint},g /etc/systemd/system/kubelet.service
sed -i s,INTERNAL_IP,$INTERNAL_IP,g /etc/systemd/system/kubelet.service
DNS_CLUSTER_IP=10.100.0.10
if [[ $INTERNAL_IP == 10.* ]] ; then DNS_CLUSTER_IP=172.20.0.10; fi
sed -i s,DNS_CLUSTER_IP,$DNS_CLUSTER_IP,g /etc/systemd/system/kubelet.service
sed -i s,CERTIFICATE_AUTHORITY_FILE,$CA_CERTIFICATE_FILE_PATH,g /var/lib/kubelet/kubeconfig
sed -i s,CLIENT_CA_FILE,$CA_CERTIFICATE_FILE_PATH,g  /etc/systemd/system/kubelet.service
systemctl daemon-reload
systemctl restart kubelet
USERDATA

}

resource "aws_launch_configuration" "testground" {
  iam_instance_profile        = aws_iam_instance_profile.testground-node.name
  image_id                    = data.aws_ami.eks_node_ami.id
  instance_type               = var.cluster-testground-instance-type
  name_prefix                 = var.cluster-name
  security_groups             = [aws_security_group.testground-node.id]
  user_data_base64            = base64encode(local.testground-node-userdata)
  key_name                    = var.cluster-key-name
  associate_public_ip_address = true

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group" "testground" {
  desired_capacity     = var.cluster-testground-desired-capacity
  launch_configuration = aws_launch_configuration.testground.id
  min_size             = var.cluster-testground-desired-capacity
  max_size             = var.cluster-testground-desired-capacity
  name                 = var.cluster-name
  vpc_zone_identifier  = aws_subnet.testground.*.id

  tag {
    key                 = "Name"
    value               = var.cluster-name
    propagate_at_launch = true
  }

  tag {
    key                 = "kubernetes.io/cluster/${var.cluster-name}"
    value               = "owned"
    propagate_at_launch = true
  }
}

