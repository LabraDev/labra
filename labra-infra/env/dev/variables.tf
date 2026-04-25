variable "project_name" {
  type    = string
  default = "labra-infra"
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "component" {
  type    = string
  default = "platform"
}

variable "aws_region" {
  type    = string
  default = "us-west-1"

  validation {
    condition     = can(regex("^[a-z]{2}(-[a-z]+)+-[0-9]+$", var.aws_region))
    error_message = "aws_region must look like us-west-1."
  }
}

variable "owner" {
  type    = string
  default = "labra-infra"
}

variable "cost_center" {
  type    = string
  default = "cpsc465"
}

variable "extra_tags" {
  type    = map(string)
  default = {}
}

variable "roadmap_phase" {
  type    = string
  default = "Phase 0"
}

variable "roadmap_version" {
  type    = string
  default = "Ver 0.1"
}

variable "bootstrap_state_backend" {
  type    = bool
  default = false
}

variable "state_bucket_name" {
  type = string
}

variable "state_lock_table_name" {
  type    = string
  default = null
}

variable "state_bucket_force_destroy" {
  type    = bool
  default = false
}

variable "enable_foundation_modules" {
  type    = bool
  default = true
}

variable "vpc_cidr" {
  type    = string
  default = "10.42.0.0/16"
}

variable "vpc_az_count" {
  type    = number
  default = 2
}

variable "vpc_enable_nat_gateway" {
  type    = bool
  default = false
}

variable "kms_enable_key_rotation" {
  type    = bool
  default = true
}

variable "kms_deletion_window_in_days" {
  type    = number
  default = 7
}

variable "logging_retention_days" {
  type    = number
  default = 14
}

variable "logging_group_suffixes" {
  type    = list(string)
  default = ["api", "deploy-runner", "webhook", "auth"]
}

variable "secrets_create_placeholder_secret" {
  type    = bool
  default = true
}

variable "enable_cognito_baseline" {
  type    = bool
  default = true
}

variable "cognito_callback_urls" {
  type    = list(string)
  default = ["https://app.example.com/dashboard"]
}

variable "cognito_logout_urls" {
  type    = list(string)
  default = ["https://app.example.com/login"]
}

variable "cognito_create_domain" {
  type    = bool
  default = true
}

variable "cognito_domain_prefix" {
  type    = string
  default = null
}

variable "enable_control_plane_cluster" {
  type    = bool
  default = true
}

variable "enable_ecr_baseline" {
  type    = bool
  default = true
}

variable "ecr_repository_names" {
  type = list(string)
  default = [
    "control-api",
    "deploy-orchestrator",
    "webhook-ingestor",
    "deploy-runner",
    "ai-runtime"
  ]
}

variable "ecr_scan_on_push" {
  type    = bool
  default = true
}

variable "ecr_mutable_tags" {
  type    = bool
  default = false
}

variable "ecr_max_images_per_repo" {
  type    = number
  default = 200
}

variable "enable_control_plane_services_baseline" {
  type    = bool
  default = false
}

variable "control_api_container_image" {
  type    = string
  default = "public.ecr.aws/nginx/nginx:stable"
}

variable "deploy_orchestrator_container_image" {
  type    = string
  default = "public.ecr.aws/docker/library/busybox:stable"
}

variable "webhook_ingestor_container_image" {
  type    = string
  default = "public.ecr.aws/docker/library/busybox:stable"
}

variable "control_api_container_port" {
  type    = number
  default = 8080
}

variable "control_api_health_check_path" {
  type    = string
  default = "/health"
}

variable "control_api_app_env" {
  type    = string
  default = "prod"
}

variable "control_api_host" {
  type    = string
  default = "0.0.0.0"
}

variable "control_api_db_url" {
  type    = string
  default = ""
}

variable "enable_control_api_db_storage" {
  type    = bool
  default = true
}

variable "control_api_db_mount_path" {
  type    = string
  default = "/mnt/labra-db"
}

variable "control_api_jwt_issuer" {
  type    = string
  default = "labra-cloud-issuer"
}

variable "control_api_jwt_audience" {
  type    = string
  default = "labra-cloud-audience"
}

variable "control_api_jwt_signing_secret" {
  type    = string
  default = "replace-with-cloud-jwt-secret"
}

variable "control_api_github_webhook_secret" {
  type    = string
  default = "replace-with-cloud-webhook-secret"
}

variable "control_api_github_oauth_redirect_url" {
  type    = string
  default = ""
}

variable "control_api_oauth_secret_arn" {
  type    = string
  default = ""
}

variable "control_api_github_client_id_secret_key" {
  type    = string
  default = "GH_CLIENT_ID"
}

variable "control_api_github_client_secret_secret_key" {
  type    = string
  default = "GH_CLIENT_SECRET"
}

variable "control_api_github_app_id_secret_key" {
  type    = string
  default = "GH_APP_ID"
}

variable "control_api_github_app_slug_secret_key" {
  type    = string
  default = "GH_APP_SLUG"
}

variable "control_api_github_app_private_key_secret_key" {
  type    = string
  default = "GH_APP_PRIVATE_KEY_PEM"
}

variable "control_api_github_webhook_secret_secret_key" {
  type    = string
  default = "GITHUB_WEBHOOK_SECRET"
}

variable "control_api_ai_prompt_version" {
  type    = string
  default = "phase7-v1"
}

variable "control_api_ai_provider_model" {
  type    = string
  default = "mock-ops-v1"
}

variable "control_api_desired_count" {
  type    = number
  default = 1
}

variable "control_plane_worker_desired_count" {
  type    = number
  default = 1
}

variable "control_plane_task_cpu" {
  type    = number
  default = 256
}

variable "control_plane_task_memory" {
  type    = number
  default = 512
}

variable "control_plane_assign_public_ip" {
  type    = bool
  default = false
}

variable "control_plane_create_execution_role" {
  type    = bool
  default = true
}

variable "control_plane_execution_role_arn" {
  type    = string
  default = null
}

variable "enable_cloudtrail_baseline" {
  type    = bool
  default = false
}

variable "cloudtrail_name" {
  type    = string
  default = null
}

variable "cloudtrail_is_multi_region_trail" {
  type    = bool
  default = true
}

variable "cloudtrail_include_global_service_events" {
  type    = bool
  default = true
}

variable "cloudtrail_enable_log_file_validation" {
  type    = bool
  default = true
}

variable "cloudtrail_kms_key_id" {
  type    = string
  default = null
}

variable "cloudtrail_force_destroy" {
  type    = bool
  default = false
}

variable "enable_waf_regional_baseline" {
  type    = bool
  default = true
}

variable "enable_waf_cloudfront_baseline" {
  type    = bool
  default = true
}

variable "enable_ai_runtime_baseline" {
  type    = bool
  default = true
}

variable "ai_feature_enabled" {
  type    = bool
  default = true
}

variable "ai_kill_switch_enabled" {
  type    = bool
  default = false
}

variable "ai_allowed_model_arns" {
  type    = list(string)
  default = ["*"]
}

variable "ai_log_retention_days" {
  type    = number
  default = 30
}

variable "enable_deployment_messaging" {
  type    = bool
  default = true
}

variable "deployment_queue_enable_alarms" {
  type    = bool
  default = true
}

variable "deployment_queue_alarm_threshold" {
  type    = number
  default = 10
}

variable "deployment_queue_alarm_period_seconds" {
  type    = number
  default = 300
}

variable "deployment_queue_alarm_evaluation_periods" {
  type    = number
  default = 1
}

variable "iam_enable_github_oidc_role" {
  type    = bool
  default = false
}

variable "iam_create_github_oidc_provider" {
  type    = bool
  default = false
}

variable "iam_github_oidc_provider_arn" {
  type    = string
  default = null
}

variable "iam_github_oidc_client_ids" {
  type    = list(string)
  default = ["sts.amazonaws.com"]
}

variable "iam_github_oidc_thumbprints" {
  type = list(string)
  default = [
    "6938fd4d98bab03faadb97b34396831e3780aea1"
  ]
}

variable "iam_github_repository" {
  type    = string
  default = null
}

variable "app_name" {
  type    = string
  default = "demo-app"
}

variable "build_type" {
  type    = string
  default = "static"
}

variable "static_site_bucket_name" {
  type    = string
  default = null
}

variable "static_default_root_object" {
  type    = string
  default = "index.html"
}

variable "static_enable_spa_routing" {
  type    = bool
  default = true
}

variable "static_price_class" {
  type    = string
  default = "PriceClass_100"
}

variable "static_force_destroy" {
  type    = bool
  default = false
}

variable "static_release_prefix" {
  type    = string
  default = "releases/"
}

variable "static_release_retention_days" {
  type    = number
  default = 90
}

variable "static_noncurrent_retention_days" {
  type    = number
  default = 30
}

variable "static_enable_alarms" {
  type    = bool
  default = true
}

variable "static_alarm_period_seconds" {
  type    = number
  default = 300

  validation {
    condition     = var.static_alarm_period_seconds > 0
    error_message = "static_alarm_period_seconds must be > 0."
  }
}

variable "static_alarm_evaluation_periods" {
  type    = number
  default = 1
}

variable "static_cf_5xx_rate_threshold" {
  type    = number
  default = 1
}
