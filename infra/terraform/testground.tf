# Specify the provider and access details
provider "aws" {
  region = var.aws_region
}

# Create a VPC to launch our instances into
resource "aws_vpc" "default" {
  cidr_block           = "172.16.0.0/16"
  enable_dns_hostnames = true
}

# Create an internet gateway to give our subnet access to the outside world
resource "aws_internet_gateway" "default" {
  vpc_id = aws_vpc.default.id
}

# Grant the VPC internet access on its main route table
resource "aws_route" "internet_access" {
  route_table_id         = aws_vpc.default.main_route_table_id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.default.id
}

# Create a subnet to launch our instances into
resource "aws_subnet" "default" {
  vpc_id                  = aws_vpc.default.id
  cidr_block              = "172.16.0.0/16"
  availability_zone       = var.aws_availability_zone
  map_public_ip_on_launch = true
}

resource "aws_security_group" "default" {
  name        = "testground"
  description = "Testground network security rules"
  vpc_id      = aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["172.16.0.0/16"]
  }

  ingress {
    from_port   = 60000
    to_port     = 61000
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "mosh"
  }

  # outbound internet access
  egress {
    from_port = 0

    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

data "aws_ami" "base" {
  most_recent = true
  owners      = ["self"]
  filter {
    name   = "tag:Name"
    values = [var.aws_ami]
  }
}

resource "aws_instance" "testground_manager" {
  ami                    = data.aws_ami.base.id
  availability_zone      = var.aws_availability_zone
  instance_type          = var.aws_instance_type_manager
  key_name               = var.key_name
  vpc_security_group_ids = ["${aws_security_group.default.id}"]
  iam_instance_profile   = "EC2-Full-API-Access-Dangerous"
  subnet_id              = aws_subnet.default.id
  private_ip             = "172.16.0.10"

  tags = {
    TG     = var.tag
    Name   = "${var.tag}-manager"
    TGRole = "manager"
  }
}

resource "aws_instance" "testground_redis" {
  ami                    = data.aws_ami.base.id
  availability_zone      = var.aws_availability_zone
  instance_type          = var.aws_instance_type_redis
  key_name               = var.key_name
  vpc_security_group_ids = ["${aws_security_group.default.id}"]
  iam_instance_profile   = "EC2-Full-API-Access-Dangerous"
  subnet_id              = aws_subnet.default.id
  private_ip             = "172.16.0.11"

  tags = {
    TG     = var.tag
    Name   = "${var.tag}-redis"
    TGRole = "redis"
  }
}

module "asg" {
  source = "terraform-aws-modules/autoscaling/aws"

  name = "service"

  # Launch configuration
  lc_name = var.tag

  image_id             = data.aws_ami.base.id
  instance_type        = var.aws_instance_type_worker
  key_name             = var.key_name
  security_groups      = ["${aws_security_group.default.id}"]
  iam_instance_profile = "EC2-Full-API-Access-Dangerous"

  # Auto scaling group
  asg_name            = var.tag
  vpc_zone_identifier = ["${aws_subnet.default.id}"]
  health_check_type   = "EC2"
  min_size            = var.workers
  max_size            = var.workers
  desired_capacity    = var.workers

  tags_as_map = {
    TG     = var.tag
    Name   = "${var.tag}-worker"
    TGRole = "worker"
  }
}
