terraform {
  backend "s3" {
    bucket = "terraform-backend-testground"
    key    = "assets-s3-bucket.tfstate"
    region = "eu-central-1"
  }
}

provider "aws" {
  region  = "eu-central-1"
  version = "~> 2.7"
}

resource "aws_s3_bucket" "assets-s3-bucket" {
  bucket = "assets-s3-bucket"
  acl    = "private"

  lifecycle_rule {
    id      = "expiration-rule"
    enabled = true

    expiration {
      days = 5
    }
  }
}

resource "aws_iam_user" "assets-s3-bucket" {
  name = "assets-s3-bucket"
}

resource "aws_iam_access_key" "assets-s3-bucket" {
  user = aws_iam_user.assets-s3-bucket.name
}

resource "aws_iam_user_policy" "assets-s3-bucket" {
  name = "assets-s3-bucket"
  user = aws_iam_user.assets-s3-bucket.name

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                        "s3:GetBucketLocation",
                        "s3:ListAllMyBuckets"
                      ],
            "Resource": "arn:aws:s3:::*"
        },
        {
            "Effect": "Allow",
            "Action": "s3:*",
            "Resource": [
                "arn:aws:s3:::assets-s3-bucket",
                "arn:aws:s3:::assets-s3-bucket/*"
            ]
        }
    ]
}
EOF
}
