output "filesystem_id" {
  value       = join("", aws_efs_file_system.default.*.id)
}

output "dns_name" {
  value       = join("", aws_efs_file_system.default.*.dns_name)
}

output "mount_target_dns_names" {
  value       = coalescelist(aws_efs_mount_target.default.*.dns_name, [""])
}

output "mount_target_ids" {
  value       = coalescelist(aws_efs_mount_target.default.*.id, [""])
}

output "mount_target_ips" {
  value       = coalescelist(aws_efs_mount_target.default.*.ip_address, [""])
}

output "network_interface_ids" {
  value       = coalescelist(aws_efs_mount_target.default.*.network_interface_id, [""])
}
