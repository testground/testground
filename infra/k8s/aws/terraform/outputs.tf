locals {
  config-map-aws-auth = <<CONFIGMAPAWSAUTH


apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth
  namespace: kube-system
data:
  mapRoles: |
    - rolearn: ${aws_iam_role.node-public.arn}
      username: system:node:{{EC2PrivateDNSName}}
      groups:
        - system:bootstrappers
        - system:nodes
  mapUsers: |
    - userarn: <user_arn, e.g. arn:aws:iam::909427826938:user/YOUR_USERNAME, click on your user in https://console.aws.amazon.com/iam/home?#/users, and copy it>
      username: <your_aws_username>
      groups:
        - system:masters


CONFIGMAPAWSAUTH


  kubeca   = aws_eks_cluster.cluster.certificate_authority[0].data
  kubehost = aws_eks_cluster.cluster.endpoint
}

output "config-map-aws-auth" {
  value = local.config-map-aws-auth
}

output "kubeca" {
  value = local.kubeca
}

output "kubehost" {
  value = local.kubehost
}
