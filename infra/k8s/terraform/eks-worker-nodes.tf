# EKS Worker Nodes Resources
#  * IAM roles:
#     - allowing Kubernetes actions to access other AWS services
#     - allowing to change autoscaling groups. Used by cluster-autoscaler
#  * EC2 Security Group to allow networking traffic
#  * Data source to fetch latest EKS worker AMI
#  * AutoScaling Launch Configuration to configure worker instances
#  * AutoScaling Group to launch worker instances

resource "aws_iam_role" "node-public" {
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

resource "aws_iam_role_policy_attachment" "node-public-AmazonEKSWorkerNodePolicy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.node-public.name
}

resource "aws_iam_role_policy_attachment" "node-public-AmazonEKS_CNI_Policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.node-public.name
}

resource "aws_iam_role_policy_attachment" "node-public-AmazonEC2ContainerRegistryReadOnly" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.node-public.name
}

resource "aws_iam_instance_profile" "node-public" {
  name = var.cluster-name
  role = aws_iam_role.node-public.name
}

## Autoscaling IAM

data "aws_iam_policy_document" "worker_autoscaling" {
  statement {
    sid    = "eksWorkerAutoscalingAll"
    effect = "Allow"

    actions = [
      "autoscaling:DescribeAutoScalingGroups",
      "autoscaling:DescribeAutoScalingInstances",
      "autoscaling:DescribeLaunchConfigurations",
      "autoscaling:DescribeTags",
    ]

    resources = ["*"]
  }

  statement {
    sid    = "eksWorkerAutoscalingOwn"
    effect = "Allow"

    actions = [
      "autoscaling:SetDesiredCapacity",
      "autoscaling:TerminateInstanceInAutoScalingGroup",
      "autoscaling:UpdateAutoScalingGroup",
    ]

    resources = ["*"]

    condition {
      test     = "StringEquals"
      variable = "autoscaling:ResourceTag/kubernetes.io/cluster/${var.cluster-name}"
      values   = ["owned"]
    }

    condition {
      test     = "StringEquals"
      variable = "autoscaling:ResourceTag/k8s.io/cluster-autoscaler/enabled"
      values   = ["true"]
    }
  }
}

resource "aws_iam_policy" "worker_autoscaling" {
  name_prefix = "eks-worker-autoscaling-${var.cluster-name}"
  description = "EKS worker node autoscaling policy for cluster ${var.cluster-name}"
  policy      = data.aws_iam_policy_document.worker_autoscaling.json
}

resource "aws_iam_role_policy_attachment" "node-public_autoscaling" {
  policy_arn = aws_iam_policy.worker_autoscaling.arn
  role       = aws_iam_role.node-public.name
}

# Security groups
resource "aws_security_group" "node-public" {
  name        = "${var.cluster-name}-node"
  description = "Security group for all nodes in the cluster"
  vpc_id      = aws_vpc.vpc.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.default_tags,
    {
      "Name"                                      = "${var.cluster-name}-node"
      "kubernetes.io/cluster/${var.cluster-name}" = "owned"
    },
  )
}

resource "aws_security_group_rule" "node-public-ingress-self" {
  description              = "Allow node to communicate with each other"
  from_port                = 0
  protocol                 = "-1"
  security_group_id        = aws_security_group.node-public.id
  source_security_group_id = aws_security_group.node-public.id
  to_port                  = 65535
  type                     = "ingress"
}

resource "aws_security_group_rule" "node-public-ingress-cluster" {
  description              = "Allow worker Kubelets and pods to receive communication from the cluster control plane"
  from_port                = 1025
  protocol                 = "tcp"
  security_group_id        = aws_security_group.node-public.id
  source_security_group_id = aws_security_group.cluster.id
  to_port                  = 65535
  type                     = "ingress"
}

resource "aws_security_group_rule" "node-public-ingress-cluster-443" {
  description              = "Allow worker Kubelets and pods to receive communication from the cluster control plane on 443. Required by metrics-server"
  from_port                = 443
  protocol                 = "tcp"
  security_group_id        = aws_security_group.node-public.id
  source_security_group_id = aws_security_group.cluster.id
  to_port                  = 443
  type                     = "ingress"
}

resource "aws_security_group_rule" "node-public-ingress-services" {
  description       = "Allow worker Kubelets and pods to receive communication from the Internet to ports 30000-32767"
  from_port         = 30000
  to_port           = 32767
  protocol          = "tcp"
  security_group_id = aws_security_group.node-public.id
  type              = "ingress"

  cidr_blocks = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "node-public-ssh" {
  description       = "Allow SSH on worker Kubelets"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  security_group_id = aws_security_group.node-public.id
  type              = "ingress"

  cidr_blocks = ["0.0.0.0/0"]
}

## Worker node definitions
data "aws_ami" "eks_node_ami" {
  filter {
    name   = "name"
    values = ["amazon-eks-node-1.14-v20190927"]
  }

  most_recent = true
  owners      = ["602401143452"] # Amazon EKS AMI Account ID
}

# The EKS AMI provides a bootstrap script to setup and connect your worker nodes.
# More info: https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh
locals {
  eks_node_userdata = <<USERDATA
#!/bin/bash -xe
set -o xtrace

/etc/eks/bootstrap.sh \
  --apiserver-endpoint '${aws_eks_cluster.cluster.endpoint}' \
  --b64-cluster-ca '${aws_eks_cluster.cluster.certificate_authority[0].data}' \
  --use-max-pods true \
  '${var.cluster-name}'
USERDATA
}

resource "aws_launch_configuration" "eks_node_launch_configuration" {
  iam_instance_profile        = aws_iam_instance_profile.node-public.name
  image_id                    = data.aws_ami.eks_node_ami.id
  instance_type               = var.worker-instance-type
  name_prefix                 = "${var.cluster-name}-worker-node"
  security_groups             = [aws_security_group.node-public.id]
  user_data_base64            = base64encode(local.eks_node_userdata)
  key_name                    = var.key-name
  associate_public_ip_address = true

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group" "eks_node_autoscaling_group_v2" {
  # NOTE: You might need to update the desired_capacity. We do have the cluster-autoscaler
  # running within k8s. This component adjusts the desired_capacity automatically based on resource usage.
  # Make sure that you adjust the desired_ capacity to whatever is currently defined in the ASG. (hint: use terraform plan)
  desired_capacity = 5

  launch_configuration = aws_launch_configuration.eks_node_launch_configuration.id
  max_size             = 20
  min_size             = 1
  name                 = "${var.cluster-name}-worker-node"
  vpc_zone_identifier  = aws_subnet.subnet.*.id

  tag {
    key                 = "Name"
    value               = "${var.cluster-name}-worker-node"
    propagate_at_launch = true
  }

  tag {
    key                 = "kubernetes.io/cluster/${var.cluster-name}"
    value               = "owned"
    propagate_at_launch = true
  }

  tag {
    key                 = "k8s.io/cluster-autoscaler/enabled"
    value               = "true"
    propagate_at_launch = false
  }

  tag {
    key                 = "Team"
    value               = "testground"
    propagate_at_launch = true
  }

  tag {
    key                 = "Environment"
    value               = "test"
    propagate_at_launch = true
  }
}
