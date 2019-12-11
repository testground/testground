variable "cluster-name" {
  type = string
}

variable "cluster-region" {
  type = string
}

variable "key-name" {
  type = string
}

variable "public-key" {
  type = string
}

variable "default_tags" {
  type = map(string)

  default = {
    Environment = "test"
    Team        = "testground"
  }
}

# see /etc/eks/eni-max-pods.txt for max pods per instance type
# m5.2xlarge - 8  vCPU 32 GB RAM - max 58 pods per instance - ~550MB per pod
# m5.4xlarge - 16 vCPU 64 GB RAM - max 234 pods per instance - ~270MB per pod
variable "worker-instance-type" {
  type    = string
  default = "m5.4xlarge"
}
