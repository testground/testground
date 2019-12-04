variable "cluster" {
  description = "Used to set AWS TestGround cluster name"
}

variable "key_name" {
  description = "Name for your AWS key pair"
}

variable "public_key" {
  description = "Your SSH public key"
}

variable "workers" {
  description = "Number of worker nodes"
}

variable "aws_region" {
  description = "AWS region to launch servers."
  default     = "eu-central-1"
}

variable "aws_availability_zone" {
  description = "AWS availability zone to launch servers."
  default     = "eu-central-1a"
}

variable "aws_ami" {
  description = "Tag for AWS AMI"
  default     = "tg-base-v1"
}

# Information about the different types of instances
# https://www.ec2instances.info/?region=us-west-2

variable "aws_instance_type_manager" {
  description = "AWS Instance type for manager node"
  default     = "c5.large"
}

variable "aws_instance_type_redis" {
  description = "AWS Instance type for redis node"
  default     = "c5.large"
}

variable "aws_instance_type_worker" {
  description = "AWS Instance type for worker node"
  default     = "m5.xlarge"
}
