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
    - userarn: arn:aws:iam::909427826938:user/anton
      username: anton
      groups:
        - system:masters
    - userarn: arn:aws:iam::909427826938:user/daviddias
      username: anton
      groups:
        - system:masters
    - userarn: arn:aws:iam::909427826938:user/dirkmc
      username: anton
      groups:
        - system:masters
    - userarn: arn:aws:iam::909427826938:user/jim
      username: anton
      groups:
        - system:masters
    - userarn: arn:aws:iam::909427826938:user/raulk
      username: anton
      groups:
        - system:masters
    - userarn: arn:aws:iam::909427826938:user/Stebalien
      username: anton
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

