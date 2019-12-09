output "manager_dns" {
  value = "${aws_instance.manager.public_dns}"
}
