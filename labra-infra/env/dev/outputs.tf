output "resource_prefix" {
  value = local.resource_prefix
}

output "tags" {
  value = local.tags
}

output "roadmap_phase" {
  value = var.roadmap_phase
}

output "roadmap_version" {
  value = var.roadmap_version
}

output "app_name" {
  value = var.app_name
}

output "build_type" {
  value = var.build_type
}

output "static_bucket_name" {
  value = module.static_runtime.bucket_name
}

output "static_distribution_id" {
  value = module.static_runtime.distribution_id
}

output "static_distribution_domain_name" {
  value = module.static_runtime.distribution_domain_name
}

output "static_distribution_hosted_zone_id" {
  value = module.static_runtime.distribution_hosted_zone_id
}

output "static_site_url" {
  value = module.static_runtime.site_url
}

output "static_release_prefix" {
  value = module.static_runtime.release_prefix
}

output "static_alarm_names" {
  value = module.static_runtime.alarm_names
}

output "kms_key_arn" {
  value = try(module.kms_baseline[0].kms_key_arn, null)
}

output "log_group_names" {
  value = try(module.logging_baseline[0].log_group_names, [])
}

output "vpc_id" {
  value = try(module.vpc_baseline[0].vpc_id, null)
}

output "vpc_public_subnet_ids" {
  value = try(module.vpc_baseline[0].public_subnet_ids, [])
}

output "vpc_private_subnet_ids" {
  value = try(module.vpc_baseline[0].private_subnet_ids, [])
}

output "frontend_security_group_id" {
  value = try(module.security_groups_baseline[0].frontend_security_group_id, null)
}

output "api_security_group_id" {
  value = try(module.security_groups_baseline[0].api_security_group_id, null)
}

output "internal_security_group_id" {
  value = try(module.security_groups_baseline[0].internal_security_group_id, null)
}

output "backend_service_role_arn" {
  value = try(module.iam_baseline[0].backend_service_role_arn, null)
}

output "deploy_runner_role_arn" {
  value = try(module.iam_baseline[0].deploy_runner_role_arn, null)
}

output "github_actions_role_arn" {
  value = try(module.iam_baseline[0].github_actions_role_arn, null)
}

output "github_oidc_provider_arn" {
  value = try(module.iam_baseline[0].github_oidc_provider_arn, null)
}

output "platform_secret_arn" {
  value = try(module.secrets_baseline[0].platform_secret_arn, null)
}

output "control_plane_cluster_name" {
  value = try(module.control_plane_cluster[0].cluster_name, null)
}

output "control_plane_cluster_arn" {
  value = try(module.control_plane_cluster[0].cluster_arn, null)
}

output "control_plane_service_namespace_id" {
  value = try(module.control_plane_cluster[0].service_discovery_namespace_id, null)
}

output "control_plane_service_log_groups" {
  value = try(module.control_plane_cluster[0].service_log_group_names, [])
}

output "ecr_repository_names" {
  value = try(module.ecr_baseline[0].repository_names, {})
}

output "ecr_repository_urls" {
  value = try(module.ecr_baseline[0].repository_urls, {})
}

output "control_plane_alb_arn" {
  value = try(module.control_plane_services_baseline[0].alb_arn, null)
}

output "control_plane_alb_dns_name" {
  value = try(module.control_plane_services_baseline[0].alb_dns_name, null)
}

output "control_plane_alb_zone_id" {
  value = try(module.control_plane_services_baseline[0].alb_zone_id, null)
}

output "control_plane_service_names" {
  value = try(module.control_plane_services_baseline[0].service_names, {})
}

output "control_plane_task_definition_arns" {
  value = try(module.control_plane_services_baseline[0].task_definition_arns, {})
}

output "control_plane_task_execution_role_arn" {
  value = try(module.control_plane_services_baseline[0].task_execution_role_arn, null)
}

output "control_api_db_url" {
  value = local.control_api_effective_db_url
}

output "control_api_db_mount_path" {
  value = var.control_api_db_mount_path
}

output "control_api_db_filesystem_id" {
  value = try(module.control_api_db_storage[0].file_system_id, null)
}

output "cloudtrail_arn" {
  value = try(module.cloudtrail_baseline[0].trail_arn, null)
}

output "cloudtrail_bucket_name" {
  value = try(module.cloudtrail_baseline[0].trail_bucket_name, null)
}

output "waf_regional_web_acl_arn" {
  value = try(module.waf_regional_baseline[0].web_acl_arn, null)
}

output "waf_cloudfront_web_acl_arn" {
  value = try(module.waf_cloudfront_baseline[0].web_acl_arn, null)
}

output "ai_requests_log_group_name" {
  value = try(module.ai_runtime_baseline[0].ai_requests_log_group_name, null)
}

output "ai_feature_enabled_parameter_name" {
  value = try(module.ai_runtime_baseline[0].ai_feature_enabled_parameter_name, null)
}

output "ai_kill_switch_parameter_name" {
  value = try(module.ai_runtime_baseline[0].ai_kill_switch_parameter_name, null)
}

output "ai_runtime_role_arn" {
  value = try(module.ai_runtime_baseline[0].ai_runtime_role_arn, null)
}

output "ai_allowed_model_arns" {
  value = try(module.ai_runtime_baseline[0].allowed_model_arns, [])
}

output "deploy_jobs_queue_name" {
  value = try(module.deployment_messaging[0].deploy_jobs_queue_name, null)
}

output "deploy_jobs_queue_url" {
  value = try(module.deployment_messaging[0].deploy_jobs_queue_url, null)
}

output "deploy_jobs_queue_arn" {
  value = try(module.deployment_messaging[0].deploy_jobs_queue_arn, null)
}

output "deploy_jobs_dlq_name" {
  value = try(module.deployment_messaging[0].deploy_jobs_dlq_name, null)
}

output "deploy_jobs_dlq_url" {
  value = try(module.deployment_messaging[0].deploy_jobs_dlq_url, null)
}

output "deploy_jobs_dlq_arn" {
  value = try(module.deployment_messaging[0].deploy_jobs_dlq_arn, null)
}

output "webhook_events_queue_name" {
  value = try(module.deployment_messaging[0].webhook_events_queue_name, null)
}

output "webhook_events_queue_url" {
  value = try(module.deployment_messaging[0].webhook_events_queue_url, null)
}

output "webhook_events_queue_arn" {
  value = try(module.deployment_messaging[0].webhook_events_queue_arn, null)
}

output "webhook_events_dlq_name" {
  value = try(module.deployment_messaging[0].webhook_events_dlq_name, null)
}

output "webhook_events_dlq_url" {
  value = try(module.deployment_messaging[0].webhook_events_dlq_url, null)
}

output "webhook_events_dlq_arn" {
  value = try(module.deployment_messaging[0].webhook_events_dlq_arn, null)
}

output "deployment_queue_alarm_names" {
  value = try(module.deployment_messaging[0].alarm_names, [])
}
