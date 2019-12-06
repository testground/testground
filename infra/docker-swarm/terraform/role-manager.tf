resource "aws_instance" "manager" {
  ami                    = data.aws_ami.base.id
  availability_zone      = var.aws_availability_zone
  instance_type          = var.aws_instance_type_manager
  key_name               = aws_key_pair.key_pair.key_name
  vpc_security_group_ids = ["${aws_security_group.default.id}"]
  subnet_id              = aws_subnet.default.id

  root_block_device {
    volume_type           = "gp2"
    volume_size           = 100
    delete_on_termination = false
  }

  tags = merge(var.default_tags, map("Name", "testground-manager-${var.cluster}", "Role", "manager", "Cluster", var.cluster))
}

