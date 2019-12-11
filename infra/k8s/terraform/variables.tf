variable "cluster-name" {
  type    = string
}

variable "cluster-region" {
  type    = string
}

variable "cluster-testground-instance-type" {
  default = "m5.xlarge"
  type    = string
}

variable "cluster-testground-desired-capacity" {
  default = "5"
  type    = string
}

variable "cluster-key-name" {
  type    = string
}

