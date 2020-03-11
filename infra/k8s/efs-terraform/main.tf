provider "aws" {
  region  = var.aws_region
  version = "~> 2.50"
}

resource "aws_efs_file_system" "default" {
  count           = 1
}

resource "aws_efs_mount_target" "default" {
  count           = 1
  file_system_id  = join("", aws_efs_file_system.default.*.id)
  subnet_id       = var.fs_subnet_id
  security_groups = [var.fs_sg_id]
}
