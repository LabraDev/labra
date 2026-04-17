locals {
  unique_subnet_ids = distinct(var.subnet_ids)
}

resource "aws_efs_file_system" "sqlite" {
  creation_token                  = "${var.name_prefix}-control-api-db"
  encrypted                       = true
  performance_mode                = var.performance_mode
  throughput_mode                 = var.throughput_mode
  provisioned_throughput_in_mibps = var.throughput_mode == "provisioned" ? var.provisioned_throughput_mibps : null

  lifecycle_policy {
    transition_to_ia = var.transition_to_ia
  }

  tags = merge(var.tags, {
    Name      = "${var.name_prefix}-control-api-db"
    Component = "control-api-db-storage"
  })
}

resource "aws_efs_mount_target" "sqlite" {
  for_each = toset(local.unique_subnet_ids)

  file_system_id  = aws_efs_file_system.sqlite.id
  subnet_id       = each.key
  security_groups = [var.security_group_id]
}
