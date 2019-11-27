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

**Note**: If you are using the Protocol Labs Test Ground Infra AWS account, there will be an AMI image already published. You won't have to create one yourself.

## Instantiating the backend with Terraform

The terraform directory contains a Hashicorp Terraform configuration that can be used to provision resources needed to run a cluster on AWS - the EC2 virtual machines, an autoscaling group for the workers, plus a VPC (Virtual Private Cloud) for the network.

The default cluster sets up a "manager" machine, another machine to run redis, and 2 worker machines for running tests on.

Steps:

1. Install the Terraform CLI [Terraform](https://www.terraform.io/).
2. Create a `~/.aws` folder to store your AWS credentials (Access Key ID and Secret Access Key). See how [here].(https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html)
3. Register a SSH Key Pair at the EC2 dashboard in the AWS web console ([EC2 Dashboard, Network & Security](https://us-west-2.console.aws.amazon.com/ec2/home?region=us-west-2#KeyPairs:sort=keyName)).
4. `cd terraform` and run `terraform init` to install all the deps necessary
5. Create a file with the name `terraform.tfvars`. It should look like:

```
key_name = "<ssh key pair name registered in AWS>"
tag      = "<name for your cluster, use only alphanumeric chars and underscores>"
```

The tag is used to name your cluster. It must be unique. Be careful not to re-use a tag that is already in-use, or your cluster might get joined into another one.

6. To set up the resources on AWS, simple run `terraform apply`. Terraform will ask for you to type in `yes` as a confirmation step. The final output from Terraform will contain the public DNS name you can ssh to get into the manager node.

Other notes:

- When you ssh, make sure you log in as the `ubuntu` user, and use ssh agent forwarding eg: `ssh -A ubuntu@<public IP address>`
- You will need to specify the right key to the ssh command, a convinient way to do this is by telling your ssh-agent about it with `ssh-add  <key-file>`)
- You can always run `terraform output` to get the address again from the local terraform state
- Use `terraform destroy` to remove the cluster from AWS

## Configuring the backend with Ansible

For now, the following steps are necessary to configure the cluster:

1. ssh to the manager machine
2. `cd ~/testground-aws-setup/infra/`
3. `git pull` (get latest scripts)
4. `cd ansible`
5. `./list-hosts.sh` (confirm that all the machines are there)
6. `./ping-all.sh` (confirm that there is connectivity to all the machines)
7. `./make-inventory.sh` (generated inventory.ini file)
8. `./setup-filebeat.sh`
9. `./setup-docker-swarm.sh`

At this point, the cluster should be ready for use.

You can now follow the steps on [Run on Cloud Infra](../README.md#running-a-test-plan-on-the-testground-cloud-infrastructure) to learn how to link your local TestGround with the remote Docker Swarm cluster and run your tests there.
