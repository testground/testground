resource "aws_instance" "worker" {
  count                  = var.workers
  ami                    = data.aws_ami.base.id
  availability_zone      = var.aws_availability_zone
  instance_type          = var.aws_instance_type_worker
  key_name               = aws_key_pair.key_pair.key_name
  vpc_security_group_ids = ["${aws_security_group.default.id}"]
  subnet_id              = aws_subnet.default.id
  iam_instance_profile   = aws_iam_instance_profile.instance_profile.name

  tags = merge(var.default_tags, map("Name", "testground-worker-${var.cluster}-${count.index + 1}", "Role", "worker", "Cluster", var.cluster))
}
