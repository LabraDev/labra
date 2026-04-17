output "file_system_id" {
  value = aws_efs_file_system.sqlite.id
}

output "file_system_arn" {
  value = aws_efs_file_system.sqlite.arn
}

output "mount_target_ids" {
  value = [for mt in aws_efs_mount_target.sqlite : mt.id]
}
