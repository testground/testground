# Setting up a Docker Swarm cluster on AWS for Testground

In this directory, you will find:

```
» tree -L 1
.
├── README.md
├── ansible     # Playbooks used to configure each of the instances
└── terraform   # Playbooks used to setup AWS infrastructure - EC2 instances, security groups, networks, etc.
```

## Requirements

- 1. [Terraform](https://www.terraform.io/).
- 2. [Ansible](https://www.ansible.com/).
- 3. [op](https://support.1password.com/command-line-getting-started/).
- 4. [terraform-inventory](https://github.com/adammck/terraform-inventory).

## Set up infrastructure with Terraform

The playbooks set up a "manager" machine, another machine to run Redis, and two worker machines for running tests on.

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

7. Use `terraform destroy` to remove the cluster from AWS when you are finished working on it.

## Generate Ansible inventory from Terraform state

1. Run

```
terraform-inventory -inventory > ../ansible/inventories/$CLUSTER_NAME
```

## Configure your cluster with Ansible

1. Sign into your 1Password account

```
eval $(op signin protocollabs)
```

2. Configure Ansible

- Copy `ansible.cfg-example` to `ansible.cfg`
- Update default inventory and private key file in `ansible.cfg`

3. Execute the setup playbook

```
ansible-playbook setup.yaml
```

---

At this point, the cluster should be ready for use.

You can now follow the steps on [Run on Cloud Infra](../README.md#running-a-test-plan-on-the-testground-cloud-infrastructure) to learn how to link your local TestGround with the remote Docker Swarm cluster and run your tests there.

## Useful commands

- Perform a quick connectivity test

```
ansible all -m ping
```

- Install required external roles and vendor them if you change a version

```
ansible-galaxy install -r roles/external/requirements.yaml -p roles/external
```
