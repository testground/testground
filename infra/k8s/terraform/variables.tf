variable "cluster-name" {
  type    = string
}

variable "cluster-region" {
  type    = string
}

variable "key-name" {
  type    = string
}

variable "public-key" {
  type    = string
}

variable "default_tags" {
  type = map(string)

  default = {
    Environment = "test"
    Team        = "testground"
  }
}
