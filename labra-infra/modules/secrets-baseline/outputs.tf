output "platform_secret_arn" {
  value = var.create_platform_secret ? aws_secretsmanager_secret.platform[0].arn : null
}

output "platform_secret_name" {
  value = var.create_platform_secret ? aws_secretsmanager_secret.platform[0].name : null
}
