# Setting up a Kubernetes cluster on AWS for Testground

In this directory, you will find:

```
» tree
.
├── README-aws.md
└── aws
    └── terraform          # Playbooks used to setup AWS EKS cluster for Testground - EC2 instances, security groups, networks, etc.
```

## Requirements

- 1. [Terraform](https://www.terraform.io/).

## Set up infrastructure with Terraform

1. [Configure your AWS credentials](https://docs.aws.amazon.com/cli/)

2. Pick a cluster name. Cluster names must be unique within the same AWS account.

```
export CLUSTER_NAME=demo
```

3. Configure the Terraform backend

- Copy `backends/example-backend.tf` to `backends/$CLUSTER_NAME.tf`
- Update `key` value in `backends/$CLUSTER_NAME.tf`

4. Initialise the Terraform backend

```
terraform init -backend-config=backends/$CLUSTER_NAME.tf
```

5. Configure your cluster

- Copy `terraform.tfvars-example` to `terraform.tfvars`
- Update vars

6. Plan and apply a new cluster with Terraform

```
terraform plan
```

```
terraform apply
```

7. Update your local .kube config and context

```
aws eks update-kubeconfig --name $CLUSTER_NAME
```

8. Export Terraform outputs

```
terraform output config-map-aws-auth > outputs/config-map-aws-auth.yaml
```

9. Apply config to Kubernetes cluster

```
kubectl apply -f ./outputs/config-map-aws-auth.yaml
```

## Teardown

Use `terraform destroy` to remove the cluster from AWS when you are finished working on it.
