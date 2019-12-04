terraform {
  backend "s3" {
    bucket = "terraform-backend-testground"
    key    = "dns/testground.ipfs.team/backend.tfstate"
    region = "eu-central-1"
  }
}

provider "aws" {
  region  = "eu-central-1"
  version = "~> 2.41"
}

variable "default_tags" {
  type = map

  default = {
    Environment = "production"
    Team        = "testground"
  }
}

resource "aws_route53_zone" "testground_ipfs_team" {
  name = "testground.ipfs.team"
  tags = merge(var.default_tags)
}

output "testground_ipfs_team_zone_id" {
  value = "${aws_route53_zone.testground_ipfs_team.zone_id}"
}

