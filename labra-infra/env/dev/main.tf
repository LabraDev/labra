terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = local.tags
  }
}

provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"

  default_tags {
    tags = local.tags
  }
}

locals {
  component_suffix             = var.component == "" ? "" : "-${var.component}"
  resource_prefix              = "${var.project_name}-${var.environment}${local.component_suffix}"
  control_api_db_mount_path    = trimspace(var.control_api_db_mount_path)
  control_api_effective_db_url = trimspace(var.control_api_db_url) != "" ? trimspace(var.control_api_db_url) : "${local.control_api_db_mount_path}/labra.db"
  tags = merge({
    Project      = var.project_name
    Environment  = var.environment
    Owner        = var.owner
    CostCenter   = var.cost_center
    ManagedBy    = "Terraform"
    Version      = var.roadmap_version
    RoadmapPhase = var.roadmap_phase
  }, var.extra_tags)
}

module "state_bootstrap" {
  count  = var.bootstrap_state_backend ? 1 : 0
  source = "../../modules/state-bootstrap"

  name_prefix       = local.resource_prefix
  state_bucket_name = var.state_bucket_name
  lock_table_name   = var.state_lock_table_name
  force_destroy     = var.state_bucket_force_destroy
  tags              = local.tags
}

module "kms_baseline" {
  count  = var.enable_foundation_modules ? 1 : 0
  source = "../../modules/kms-baseline"

  name_prefix             = local.resource_prefix
  enable_key_rotation     = var.kms_enable_key_rotation
  deletion_window_in_days = var.kms_deletion_window_in_days
  tags                    = local.tags
}

module "logging_baseline" {
  count  = var.enable_foundation_modules ? 1 : 0
  source = "../../modules/logging-baseline"

  name_prefix        = local.resource_prefix
  log_retention_days = var.logging_retention_days
  log_group_suffixes = var.logging_group_suffixes
  tags               = local.tags
}

module "vpc_baseline" {
  count  = var.enable_foundation_modules ? 1 : 0
  source = "../../modules/vpc-baseline"

  name_prefix        = local.resource_prefix
  vpc_cidr           = var.vpc_cidr
  az_count           = var.vpc_az_count
  enable_nat_gateway = var.vpc_enable_nat_gateway
  tags               = local.tags
}

module "security_groups_baseline" {
  count  = var.enable_foundation_modules ? 1 : 0
  source = "../../modules/security-groups-baseline"

  name_prefix = local.resource_prefix
  vpc_id      = module.vpc_baseline[0].vpc_id
  tags        = local.tags
}

module "iam_baseline" {
  count  = var.enable_foundation_modules ? 1 : 0
  source = "../../modules/iam-baseline"

  name_prefix                 = local.resource_prefix
  enable_github_oidc_role     = var.iam_enable_github_oidc_role
  create_github_oidc_provider = var.iam_create_github_oidc_provider
  github_oidc_provider_arn    = var.iam_github_oidc_provider_arn
  github_oidc_client_ids      = var.iam_github_oidc_client_ids
  github_oidc_thumbprints     = var.iam_github_oidc_thumbprints
  github_repository           = var.iam_github_repository
  tags                        = local.tags
}

module "secrets_baseline" {
  count  = var.enable_foundation_modules ? 1 : 0
  source = "../../modules/secrets-baseline"

  name_prefix               = local.resource_prefix
  create_placeholder_secret = var.secrets_create_placeholder_secret
  tags                      = local.tags
}

module "cognito_baseline" {
  count  = var.enable_cognito_baseline ? 1 : 0
  source = "../../modules/cognito-baseline"

  name_prefix   = local.resource_prefix
  callback_urls = var.cognito_callback_urls
  logout_urls   = var.cognito_logout_urls
  create_domain = var.cognito_create_domain
  domain_prefix = var.cognito_domain_prefix
  tags          = local.tags
}

module "control_plane_cluster" {
  count  = var.enable_control_plane_cluster && var.enable_foundation_modules ? 1 : 0
  source = "../../modules/control-plane-cluster"

  name_prefix = local.resource_prefix
  vpc_id      = module.vpc_baseline[0].vpc_id
  tags        = local.tags
}

module "ecr_baseline" {
  count  = var.enable_ecr_baseline ? 1 : 0
  source = "../../modules/ecr-baseline"

  name_prefix         = local.resource_prefix
  repositories        = var.ecr_repository_names
  scan_on_push        = var.ecr_scan_on_push
  mutable_tags        = var.ecr_mutable_tags
  max_images_per_repo = var.ecr_max_images_per_repo
  tags                = local.tags
}

module "control_api_db_storage" {
  count  = var.enable_control_api_db_storage && var.enable_control_plane_services_baseline && var.enable_foundation_modules ? 1 : 0
  source = "../../modules/control-api-db-storage"

  name_prefix       = local.resource_prefix
  subnet_ids        = length(module.vpc_baseline[0].private_subnet_ids) > 0 ? module.vpc_baseline[0].private_subnet_ids : module.vpc_baseline[0].public_subnet_ids
  security_group_id = module.security_groups_baseline[0].internal_security_group_id
  tags              = local.tags
}

module "control_plane_services_baseline" {
  count  = var.enable_control_plane_services_baseline && var.enable_control_plane_cluster && var.enable_foundation_modules ? 1 : 0
  source = "../../modules/control-plane-services-baseline"

  name_prefix                         = local.resource_prefix
  cluster_arn                         = module.control_plane_cluster[0].cluster_arn
  vpc_id                              = module.vpc_baseline[0].vpc_id
  public_subnet_ids                   = module.vpc_baseline[0].public_subnet_ids
  private_subnet_ids                  = module.vpc_baseline[0].private_subnet_ids
  alb_security_group_id               = module.security_groups_baseline[0].frontend_security_group_id
  api_service_security_group_id       = module.security_groups_baseline[0].api_security_group_id
  worker_service_security_group_id    = module.security_groups_baseline[0].internal_security_group_id
  api_container_image                 = var.control_api_container_image
  deploy_orchestrator_container_image = var.deploy_orchestrator_container_image
  webhook_ingestor_container_image    = var.webhook_ingestor_container_image
  api_container_port                  = var.control_api_container_port
  api_health_check_path               = var.control_api_health_check_path
  api_desired_count                   = var.control_api_desired_count
  worker_desired_count                = var.control_plane_worker_desired_count
  task_cpu                            = var.control_plane_task_cpu
  task_memory                         = var.control_plane_task_memory
  assign_public_ip                    = var.control_plane_assign_public_ip
  create_execution_role               = var.control_plane_create_execution_role
  execution_role_arn                  = var.control_plane_execution_role_arn
  api_efs_file_system_id              = var.enable_control_api_db_storage ? try(module.control_api_db_storage[0].file_system_id, null) : null
  api_db_mount_path                   = local.control_api_db_mount_path
  service_environment = {
    "control-api" = {
      APP_ENV                   = var.control_api_app_env
      API_HOST                  = var.control_api_host
      API_PORT                  = tostring(var.control_api_container_port)
      DB_URL                    = local.control_api_effective_db_url
      JWT_ISSUER                = var.control_api_jwt_issuer
      JWT_AUDIENCE              = var.control_api_jwt_audience
      JWT_SIGNING_SECRET        = var.control_api_jwt_signing_secret
      GITHUB_WEBHOOK_SECRET     = var.control_api_github_webhook_secret
      GITHUB_OAUTH_REDIRECT_URL = var.control_api_github_oauth_redirect_url
      AI_PROMPT_VERSION         = var.control_api_ai_prompt_version
      AI_PROVIDER_MODEL         = var.control_api_ai_provider_model
      AI_FEATURE_ENABLED        = tostring(var.ai_feature_enabled)
      AI_KILL_SWITCH_ENABLED    = tostring(var.ai_kill_switch_enabled)
    }
  }
  task_role_arns = {
    "control-api"         = try(module.iam_baseline[0].backend_service_role_arn, "")
    "deploy-orchestrator" = try(module.iam_baseline[0].deploy_runner_role_arn, "")
    "webhook-ingestor"    = try(module.iam_baseline[0].deploy_runner_role_arn, "")
  }
  tags = local.tags
}

module "cloudtrail_baseline" {
  count  = var.enable_cloudtrail_baseline ? 1 : 0
  source = "../../modules/cloudtrail-baseline"

  name_prefix                   = local.resource_prefix
  trail_name                    = var.cloudtrail_name
  is_multi_region_trail         = var.cloudtrail_is_multi_region_trail
  include_global_service_events = var.cloudtrail_include_global_service_events
  enable_log_file_validation    = var.cloudtrail_enable_log_file_validation
  kms_key_id                    = var.cloudtrail_kms_key_id
  force_destroy                 = var.cloudtrail_force_destroy
  tags                          = local.tags
}

module "waf_regional_baseline" {
  count  = var.enable_waf_regional_baseline && var.enable_control_plane_services_baseline ? 1 : 0
  source = "../../modules/waf-regional-baseline"

  name_prefix            = local.resource_prefix
  associate_resource_arn = var.enable_control_plane_services_baseline ? try(module.control_plane_services_baseline[0].alb_arn, null) : null
  tags                   = local.tags
}

module "waf_cloudfront_baseline" {
  count  = var.enable_waf_cloudfront_baseline ? 1 : 0
  source = "../../modules/waf-cloudfront-baseline"

  providers = {
    aws = aws.us_east_1
  }

  name_prefix = local.resource_prefix
  tags        = local.tags
}

module "ai_runtime_baseline" {
  count  = var.enable_ai_runtime_baseline ? 1 : 0
  source = "../../modules/ai-runtime-baseline"

  name_prefix         = local.resource_prefix
  feature_enabled     = var.ai_feature_enabled
  kill_switch_enabled = var.ai_kill_switch_enabled
  allowed_model_arns  = var.ai_allowed_model_arns
  log_retention_days  = var.ai_log_retention_days
  tags                = local.tags
}

module "deployment_messaging" {
  count  = var.enable_deployment_messaging ? 1 : 0
  source = "../../modules/deployment-messaging"

  name_prefix                      = local.resource_prefix
  enable_alarms                    = var.deployment_queue_enable_alarms
  alarm_visible_messages_threshold = var.deployment_queue_alarm_threshold
  alarm_period_seconds             = var.deployment_queue_alarm_period_seconds
  alarm_evaluation_periods         = var.deployment_queue_alarm_evaluation_periods
  tags                             = local.tags
}

module "static_runtime" {
  source = "../../modules/static_runtime"

  name_prefix               = local.resource_prefix
  app_name                  = var.app_name
  build_type                = var.build_type
  region                    = var.aws_region
  bucket_name               = var.static_site_bucket_name
  api_origin_domain_name    = var.enable_control_plane_services_baseline ? try(module.control_plane_services_baseline[0].alb_dns_name, null) : null
  cloudfront_web_acl_arn    = var.enable_waf_cloudfront_baseline ? try(module.waf_cloudfront_baseline[0].web_acl_arn, null) : null
  default_root_object       = var.static_default_root_object
  price_class               = var.static_price_class
  enable_spa_routing        = var.static_enable_spa_routing
  force_destroy             = var.static_force_destroy
  release_prefix            = var.static_release_prefix
  release_retention_days    = var.static_release_retention_days
  noncurrent_retention_days = var.static_noncurrent_retention_days
  enable_alarms             = var.static_enable_alarms
  alarm_period_seconds      = var.static_alarm_period_seconds
  alarm_evaluation_periods  = var.static_alarm_evaluation_periods
  cf_5xx_rate_threshold     = var.static_cf_5xx_rate_threshold
  tags                      = local.tags
}
