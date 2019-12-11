terraform {
  backend "s3" {
    bucket = "terraform-backend-testground"
    key    = "tony-eks.tfstate"
    region = "eu-central-1"
  }
}
