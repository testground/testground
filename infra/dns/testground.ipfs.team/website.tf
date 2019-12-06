variable "domain" {
  default = "testground.ipfs.team"
}

variable "redirect" {
  default = "https://github.com/ipfs/testground"
}

resource "aws_s3_bucket" "redirect" {
  bucket = var.domain
  acl    = "private"

  website {
    redirect_all_requests_to = var.redirect
  }
}

resource "aws_route53_record" "testground_ipfs_team" {
  name    = var.domain
  zone_id = aws_route53_zone.testground_ipfs_team.zone_id
  type    = "A"

  alias {
    name                   = aws_s3_bucket.redirect.website_domain
    zone_id                = aws_s3_bucket.redirect.hosted_zone_id
    evaluate_target_health = true
  }
}
