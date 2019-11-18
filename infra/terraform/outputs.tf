output "address" {
  value = "${aws_instance.testground_manager.public_dns}"
}
