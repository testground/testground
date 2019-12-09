resource "aws_security_group" "default" {
  name        = "testground-${var.cluster}"
  description = "Testground-${var.cluster} network security rules"
  vpc_id      = aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # testground daemon
  ingress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["172.16.0.0/16"]
  }

  # outbound internet access
  egress {
    from_port = 0

    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
