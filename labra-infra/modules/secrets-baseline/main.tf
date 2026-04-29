resource "aws_secretsmanager_secret" "platform" {
  count = var.create_platform_secret ? 1 : 0

  name                    = "${var.name_prefix}/platform"
  description             = "Labra platform secret envelope (populate values outside Terraform)."
  recovery_window_in_days = 7

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-platform-secret"
  })
}
