#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="$ROOT_DIR/labra-infra/env/dev"
FRONTEND_DIR="$ROOT_DIR/labra-frontend"
BACKEND_DIR="$ROOT_DIR/labra-backend"
TFVARS_FILE="$TF_DIR/terraform.tfvars"
CLOUD_TFVARS_FILE="$TF_DIR/cloud.auto.tfvars"

AUTO_YES=0
SKIP_INFRA_BOOTSTRAP=0
SKIP_FRONTEND_DEPLOY=0
SKIP_IMAGE_BUILD=0

STEP=1

usage() {
  cat <<'USAGE'
Usage: ./cloud-up.sh [options]

End-to-end cloud deployment automation for Labra:
1) Runs infra provisioning if needed
2) Builds backend container image and pushes to ECR
3) Applies ECS runtime vars for cloud API services
4) Enables persistent EFS-backed SQLite storage for cloud DB state
5) Builds frontend and uploads to S3 static bucket
6) Invalidates CloudFront and prints live cloud URLs

Options:
  --yes                  Auto-approve prompts and Terraform apply.
  --skip-infra-bootstrap Skip running infra-up.sh baseline step.
  --skip-image-build     Reuse existing image URIs from cloud.auto.tfvars.
  --skip-frontend-deploy Skip frontend build/upload/invalidation.
  -h, --help             Show help.
USAGE
}

log_step() {
  printf '\n[%02d] %s\n' "$STEP" "$1"
  STEP=$((STEP + 1))
}

log_note() {
  printf '  - %s\n' "$1"
}

die() {
  printf '\nERROR: %s\n' "$1" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

confirm_or_exit() {
  local message="$1"
  if [[ "$AUTO_YES" -eq 1 ]]; then
    log_note "Auto-approved: $message"
    return 0
  fi
  local answer
  printf '%s [y/N]: ' "$message"
  read -r answer
  case "${answer,,}" in
    y|yes) ;;
    *) die "Cancelled by user." ;;
  esac
}

tf() {
  terraform -chdir="$TF_DIR" "$@"
}

tf_out_raw() {
  local key="$1"
  tf output -raw "$key" 2>/dev/null || true
}

read_tfvar_value() {
  local key="$1"
  local line value
  line="$(grep -E "^[[:space:]]*${key}[[:space:]]*=" "$TFVARS_FILE" | tail -n 1 || true)"
  if [[ -z "$line" ]]; then
    printf ''
    return 0
  fi
  value="${line#*=}"
  value="$(printf '%s' "$value" | sed -E 's/[[:space:]]+#.*$//' | xargs)"
  value="${value%\"}"
  value="${value#\"}"
  printf '%s' "$value"
}

wait_for_health() {
  local url="$1"
  local label="$2"
  local attempts=60
  local i
  for ((i = 1; i <= attempts; i++)); do
    if curl -fsS -m 10 "$url" >/dev/null 2>&1; then
      log_note "${label} healthy: ${url}"
      return 0
    fi
    sleep 5
  done
  return 1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --yes)
        AUTO_YES=1
        ;;
      --skip-infra-bootstrap)
        SKIP_INFRA_BOOTSTRAP=1
        ;;
      --skip-image-build)
        SKIP_IMAGE_BUILD=1
        ;;
      --skip-frontend-deploy)
        SKIP_FRONTEND_DEPLOY=1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        usage
        die "Unknown argument: $1"
        ;;
    esac
    shift
  done
}

bootstrap_infra_if_needed() {
  if [[ "$SKIP_INFRA_BOOTSTRAP" -eq 1 ]]; then
    log_step "Skipping baseline infra bootstrap (--skip-infra-bootstrap)"
    return 0
  fi

  log_step "Running baseline Terraform infrastructure setup"
  if [[ "$AUTO_YES" -eq 1 ]]; then
    "$ROOT_DIR/infra-up.sh" --yes
  else
    "$ROOT_DIR/infra-up.sh"
  fi
}

build_and_push_images() {
  local aws_region aws_account image_tag
  local ecr_map api_repo worker_repo webhook_repo

  aws_region="$(read_tfvar_value "aws_region")"
  if [[ -z "$aws_region" ]]; then
    aws_region="us-west-1"
  fi
  aws_account="$(aws sts get-caller-identity --query Account --output text)"
  export AWS_DEFAULT_REGION="$aws_region"

  ecr_map="$(tf output -json ecr_repository_urls 2>/dev/null || true)"
  if [[ -z "$ecr_map" || "$ecr_map" == "null" ]]; then
    die "ecr_repository_urls output missing. Ensure enable_ecr_baseline=true and run infra-up.sh first."
  fi

  api_repo="$(printf '%s' "$ecr_map" | jq -r '."control-api" // empty')"
  worker_repo="$(printf '%s' "$ecr_map" | jq -r '."deploy-orchestrator" // empty')"
  webhook_repo="$(printf '%s' "$ecr_map" | jq -r '."webhook-ingestor" // empty')"
  [[ -n "$api_repo" ]] || die "Missing ECR repo URL for control-api"
  [[ -n "$worker_repo" ]] || die "Missing ECR repo URL for deploy-orchestrator"
  [[ -n "$webhook_repo" ]] || die "Missing ECR repo URL for webhook-ingestor"

  image_tag="$(git -C "$ROOT_DIR" rev-parse --short HEAD)-$(date +%Y%m%d%H%M%S)"

  log_step "Logging in to ECR"
  aws ecr get-login-password --region "$aws_region" | docker login --username AWS --password-stdin "${aws_account}.dkr.ecr.${aws_region}.amazonaws.com"

  log_step "Building backend API image"
  docker build -t "labra-api:${image_tag}" "$BACKEND_DIR"

  log_step "Tagging and pushing images to ECR"
  docker tag "labra-api:${image_tag}" "${api_repo}:${image_tag}"
  docker tag "labra-api:${image_tag}" "${worker_repo}:${image_tag}"
  docker tag "labra-api:${image_tag}" "${webhook_repo}:${image_tag}"

  docker push "${api_repo}:${image_tag}"
  docker push "${worker_repo}:${image_tag}"
  docker push "${webhook_repo}:${image_tag}"

  CONTROL_API_IMAGE="${api_repo}:${image_tag}"
  DEPLOY_WORKER_IMAGE="${worker_repo}:${image_tag}"
  WEBHOOK_WORKER_IMAGE="${webhook_repo}:${image_tag}"
}

write_cloud_tfvars() {
  local static_site_url jwt_secret webhook_secret oauth_redirect

  static_site_url="$(tf_out_raw static_site_url)"
  [[ -n "$static_site_url" ]] || die "static_site_url output missing after infra bootstrap."

  jwt_secret="$(openssl rand -hex 32)"
  webhook_secret="$(openssl rand -hex 32)"
  oauth_redirect="${static_site_url}/v1/callback"

  log_step "Writing cloud runtime tfvars ($CLOUD_TFVARS_FILE)"
  cat > "$CLOUD_TFVARS_FILE" <<EOF
enable_control_plane_services_baseline = true
enable_control_api_db_storage          = true
enable_waf_regional_baseline           = true
enable_waf_cloudfront_baseline         = true
control_plane_assign_public_ip         = true
control_api_container_port             = 8080
control_api_health_check_path          = "/health"
control_api_db_mount_path              = "/mnt/labra-db"

control_api_app_env                    = "prod"
control_api_host                       = "0.0.0.0"
control_api_db_url                     = "/mnt/labra-db/labra.db"
control_api_jwt_issuer                 = "labra-cloud-issuer"
control_api_jwt_audience               = "labra-cloud-audience"
control_api_jwt_signing_secret         = "${jwt_secret}"
control_api_github_webhook_secret      = "${webhook_secret}"
control_api_github_oauth_redirect_url  = "${oauth_redirect}"
control_api_ai_prompt_version          = "phase7-v1"
control_api_ai_provider_model          = "mock-ops-v1"

cognito_callback_urls                  = ["${static_site_url}/dashboard"]
cognito_logout_urls                    = ["${static_site_url}/login"]
EOF

  if [[ "$SKIP_IMAGE_BUILD" -eq 0 ]]; then
    cat >> "$CLOUD_TFVARS_FILE" <<EOF
control_api_container_image            = "${CONTROL_API_IMAGE}"
deploy_orchestrator_container_image    = "${DEPLOY_WORKER_IMAGE}"
webhook_ingestor_container_image       = "${WEBHOOK_WORKER_IMAGE}"
EOF
  fi

  log_note "Cloud runtime overrides written"
}

apply_cloud_runtime() {
  log_step "Applying cloud runtime infrastructure settings"
  export AWS_EC2_METADATA_DISABLED=true

  tf init -reconfigure -backend-config=backend.hcl
  tf validate
  tf plan -input=false -lock=false -refresh=false -var='bootstrap_state_backend=false' -var-file=cloud.auto.tfvars

  confirm_or_exit "Apply cloud runtime changes now?"
  if [[ "$AUTO_YES" -eq 1 ]]; then
    tf apply -input=false -auto-approve -var='bootstrap_state_backend=false' -var-file=cloud.auto.tfvars
  else
    tf apply -input=false -var='bootstrap_state_backend=false' -var-file=cloud.auto.tfvars
  fi
}

deploy_frontend() {
  local bucket distribution

  if [[ "$SKIP_FRONTEND_DEPLOY" -eq 1 ]]; then
    log_step "Skipping frontend deployment (--skip-frontend-deploy)"
    return 0
  fi

  bucket="$(tf_out_raw static_bucket_name)"
  distribution="$(tf_out_raw static_distribution_id)"
  [[ -n "$bucket" ]] || die "static_bucket_name output missing."
  [[ -n "$distribution" ]] || die "static_distribution_id output missing."

  log_step "Building frontend for static cloud hosting"
  (cd "$FRONTEND_DIR" && npm install && npm run build)

  [[ -d "$FRONTEND_DIR/build" ]] || die "Frontend build output not found at labra-frontend/build."

  log_step "Uploading frontend artifacts to S3"
  aws s3 sync "$FRONTEND_DIR/build/" "s3://${bucket}" --delete

  log_step "Invalidating CloudFront cache"
  aws cloudfront create-invalidation --distribution-id "$distribution" --paths "/*" >/dev/null
}

verify_cloud_endpoints() {
  local site_url alb_dns
  site_url="$(tf_out_raw static_site_url)"
  alb_dns="$(tf_out_raw control_plane_alb_dns_name)"

  log_step "Verifying cloud endpoints"
  [[ -n "$site_url" ]] || die "static_site_url output missing."
  wait_for_health "$site_url" "CloudFront frontend" || die "CloudFront endpoint did not respond in time: $site_url"

  if [[ -n "$alb_dns" && "$alb_dns" != "null" ]]; then
    wait_for_health "http://${alb_dns}/health" "ALB API" || die "ALB API health endpoint did not respond in time: http://${alb_dns}/health"
  else
    die "control_plane_alb_dns_name is empty. Ensure control_plane_services_baseline is enabled."
  fi

  printf '\nCloud deployment summary:\n'
  printf '  - Frontend URL: %s\n' "$site_url"
  printf '  - API URL: http://%s\n' "$alb_dns"
  printf '  - Cognito user pool: %s\n' "$(tf_out_raw cognito_user_pool_id)"
  printf '  - Cognito app client: %s\n' "$(tf_out_raw cognito_app_client_id)"
  printf '  - ALB WAF ARN: %s\n' "$(tf_out_raw waf_regional_web_acl_arn)"
  printf '  - CloudFront WAF ARN: %s\n' "$(tf_out_raw waf_cloudfront_web_acl_arn)"
  printf '  - Control API DB URL: %s\n' "$(tf_out_raw control_api_db_url)"
  printf '  - Control API EFS ID: %s\n' "$(tf_out_raw control_api_db_filesystem_id)"
  printf '  - Cloud runtime tfvars: %s\n' "$CLOUD_TFVARS_FILE"
}

main() {
  parse_args "$@"

  [[ -d "$TF_DIR" ]] || die "Missing Terraform directory: $TF_DIR"
  [[ -f "$TFVARS_FILE" ]] || die "Missing terraform.tfvars: $TFVARS_FILE"

  require_cmd aws
  require_cmd terraform
  require_cmd docker
  require_cmd jq
  require_cmd npm
  require_cmd openssl
  require_cmd curl

  bootstrap_infra_if_needed

  if [[ "$SKIP_IMAGE_BUILD" -eq 0 ]]; then
    build_and_push_images
  fi

  write_cloud_tfvars
  apply_cloud_runtime
  deploy_frontend
  verify_cloud_endpoints

  printf '\nDone: cloud stack is online and reachable.\n'
}

main "$@"
