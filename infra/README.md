# Setting up your AWS Backend for TestGround

In this directory, you will find:

```
» tree -L 1
.
├── README.md
├── packer      # The script used to build the base Image we will run on an AWS EC2 Machine
├── terraform   # The script used to spin up multiple EC2 instances
└── ansible     # The scripts used to configure each of the instances
```

## Building a custom Image with Packer

The packer directory contains a Hashicorp Packer template used to build the base AMI disk image for AWS. See [packer.io](https://www.packer.io/) for information on how to use Packer.

**Note: If you are using the Protocol Labs Test Ground Infra AWS account, there will be an AMI image already published. You won't have to create one yourself.

## Instantiating the backend with Terraform

The terraform directory contains a Hashicorp Terraform configuration that can be used to provision resources needed to run a cluster on AWS - the EC2 virtual machines, an autoscaling group for the workers, plus a VPC (Virtual Private Cloud) for the network.

The default cluster sets up a "manager" machine, another machine to run redis, and 2 worker machines for running tests on.

Steps:

- 1. Install the Terraform CLI [Terraform](https://www.terraform.io/).
- 2. Get some AWS credentitals (Access Key ID and Secret Access Key). If you don't have a key pair, you can create one in the EC2 dashboard in the AWS web console.
- 3. Go into the Terraform directory and create a file with the name `terraform.tfvars`. It should look like:

```
key_name = <ssh key pair name registered in AWS>
tag      = <name for your cluster, use only alphanumeric chars and underscores>
```

The tag is used to name your cluster. It must be unique. Be careful not to re-use a tag that is already in-use, or your cluster might get joined into another one.

- 4. To set up the resources on AWS, simple run `aws apply`. Terraform will ask for you to type in `yes` as a confirmation step. The final output from Terraform will contain the public DNS name you can ssh to get into the manager node.

Other notes:

When you ssh, make sure you log in as the `ubuntu` user, and use ssh agent forwarding (with your private key loaded into your ssh-agent using `ssh-add <key-file>`). eg:

```
ssh -A ubuntu@<public IP address>
```

You can always run `terraform output` to get the address again from the local terraform state.

Use `terraform destroy` to remove the cluster from AWS.

## Configuring the backend with Ansible

For now, the following steps are necessary to configure the cluster:

- 1. ssh to the manager machine
- 2. `cd ~/testground-aws-setup/infra/aws/ansible`
- 3. `git pull` (get latest scripts)
- 4. `./list-hosts.sh` (confirm that all the machines are there)
- 5. `./ping-all.sh` (confirm that there is connectivity to all the machines)
- 6. `./make-inventory.sh` (generated inventory.ini file)
- 7. `./setup-filebeat.sh`
- 8. `./setup-docker-swarm.sh`

At this point, the cluster should be ready for use.
