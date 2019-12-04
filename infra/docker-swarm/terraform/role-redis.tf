resource "aws_instance" "redis" {
  ami                    = data.aws_ami.base.id
  availability_zone      = var.aws_availability_zone
  instance_type          = var.aws_instance_type_redis
  key_name               = aws_key_pair.key_pair.key_name
  vpc_security_group_ids = ["${aws_security_group.default.id}"]
  subnet_id              = aws_subnet.default.id

  tags = merge(var.default_tags, map("Name", "testground-redis-${var.cluster}", "Role", "redis", "Cluster", var.cluster))
}
