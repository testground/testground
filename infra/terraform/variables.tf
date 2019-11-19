variable "key_name" {
  description = "Name of AWS key pair"
}

variable "aws_region" {
  description = "AWS region to launch servers."
  default     = "us-west-2"
}

variable "aws_availability_zone" {
  description = "AWS availability zone to launch servers."
  default     = "us-west-2c"
}

variable "aws_ami" {
  description = "Tag for AWS AMI"
  default     = "tg-base-v1"
}

variable "tag" {
  description = "Used to set AWS TestGround tag (TG)"
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

variable "workers" {
  description = "Number of worker nodes"
  default     = 0
}
