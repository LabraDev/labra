#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="$ROOT_DIR/labra-infra/env/dev"
TFVARS_FILE="$TF_DIR/terraform.tfvars"
BACKEND_STATE_FILE="$TF_DIR/backend-state.env"
BACKEND_HCL_FILE="$TF_DIR/backend.hcl"
BACKEND_HCL_EXAMPLE="$TF_DIR/backend.hcl.example"

AUTO_APPROVE=0
PLAN_ONLY=0
SKIP_BOOTSTRAP=0
FORCE_REGENERATE_BACKEND_HCL=0

AWS_REGION_FROM_TFVARS=""
STATE_BUCKET_NAME=""
STATE_LOCK_TABLE_NAME=""

STEP=1

usage() {
  cat <<'USAGE'
Usage: ./infra-up.sh [options]

Provision and update Labra platform infrastructure via Terraform with minimal manual work.
This script can:
1) auto-bootstrap Terraform state backend (S3 + DynamoDB),
2) initialize remote backend,
3) plan/apply platform infrastructure,
4) print key outputs for app setup and customer onboarding.

Options:
  --yes                      Skip interactive confirmation prompts.
  --plan-only                Stop after plan (no apply).
  --skip-bootstrap           Skip bootstrap logic even if state backend is missing.
  --force-backend-hcl        Regenerate backend.hcl from backend-state.env and terraform.tfvars values.
  -h, --help                 Show this help.
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

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --yes)
        AUTO_APPROVE=1
        ;;
      --plan-only)
        PLAN_ONLY=1
        ;;
      --skip-bootstrap)
        SKIP_BOOTSTRAP=1
        ;;
      --force-backend-hcl)
        FORCE_REGENERATE_BACKEND_HCL=1
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

confirm_or_exit() {
  local message="$1"
  if [[ "$AUTO_APPROVE" -eq 1 ]]; then
    log_note "Auto-approved: $message"
    return 0
  fi

  local answer
  printf '%s [y/N]: ' "$message"
  read -r answer
  case "${answer,,}" in
    y|yes)
      return 0
      ;;
    *)
      die "Cancelled by user."
      ;;
  esac
}

tf() {
  terraform -chdir="$TF_DIR" "$@"
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

load_config_values() {
  [[ -f "$TFVARS_FILE" ]] || die "Missing terraform.tfvars: $TFVARS_FILE"
  [[ -f "$BACKEND_STATE_FILE" ]] || die "Missing backend state config: $BACKEND_STATE_FILE"

  # shellcheck disable=SC1090
  source "$BACKEND_STATE_FILE"
  STATE_BUCKET_NAME="${STATE_BUCKET_NAME:-}"
  STATE_LOCK_TABLE_NAME="${STATE_LOCK_TABLE_NAME:-}"
  AWS_REGION_FROM_TFVARS="$(read_tfvar_value "aws_region")"

  if [[ -z "$STATE_BUCKET_NAME" ]]; then
    die "STATE_BUCKET_NAME is missing in $BACKEND_STATE_FILE"
  fi
  if [[ -z "$STATE_LOCK_TABLE_NAME" ]]; then
    STATE_LOCK_TABLE_NAME="labra-infra-dev-platform-terraform-locks"
    log_note "STATE_LOCK_TABLE_NAME missing in backend-state.env; defaulting to $STATE_LOCK_TABLE_NAME"
  fi
  if [[ -z "$AWS_REGION_FROM_TFVARS" ]]; then
    AWS_REGION_FROM_TFVARS="us-west-1"
  fi
}

write_backend_hcl() {
  cat > "$BACKEND_HCL_FILE" <<EOF
bucket         = "$STATE_BUCKET_NAME"
key            = "env/dev/terraform.tfstate"
region         = "$AWS_REGION_FROM_TFVARS"
dynamodb_table = "$STATE_LOCK_TABLE_NAME"
encrypt        = true
EOF
}

ensure_backend_hcl() {
  log_step "Ensuring backend.hcl is present"
  if [[ "$FORCE_REGENERATE_BACKEND_HCL" -eq 1 ]]; then
    write_backend_hcl
    log_note "Regenerated backend.hcl from backend-state.env + terraform.tfvars values"
    return 0
  fi

  if [[ ! -f "$BACKEND_HCL_FILE" ]]; then
    if [[ -f "$BACKEND_HCL_EXAMPLE" ]]; then
      write_backend_hcl
      log_note "Generated backend.hcl from backend-state.env + terraform.tfvars values"
    else
      die "Missing backend.hcl and backend.hcl.example."
    fi
  else
    log_note "Using existing backend.hcl"
  fi
}

check_aws_access() {
  log_step "Validating AWS authentication"
  aws sts get-caller-identity >/dev/null
  log_note "AWS credentials are valid"
}

create_backend_bucket_if_missing() {
  if aws s3api head-bucket --bucket "$STATE_BUCKET_NAME" >/dev/null 2>&1; then
    log_note "State bucket exists: $STATE_BUCKET_NAME"
    return 0
  fi

  log_note "Creating state bucket: $STATE_BUCKET_NAME"
  if [[ "$AWS_REGION_FROM_TFVARS" == "us-east-1" ]]; then
    aws s3api create-bucket --bucket "$STATE_BUCKET_NAME" >/dev/null
  else
    aws s3api create-bucket \
      --bucket "$STATE_BUCKET_NAME" \
      --create-bucket-configuration LocationConstraint="$AWS_REGION_FROM_TFVARS" >/dev/null
  fi

  aws s3api put-public-access-block \
    --bucket "$STATE_BUCKET_NAME" \
    --public-access-block-configuration \
    BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true >/dev/null

  aws s3api put-bucket-versioning \
    --bucket "$STATE_BUCKET_NAME" \
    --versioning-configuration Status=Enabled >/dev/null

  aws s3api put-bucket-encryption \
    --bucket "$STATE_BUCKET_NAME" \
    --server-side-encryption-configuration \
    '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}' >/dev/null
}

create_backend_lock_table_if_missing() {
  if aws dynamodb describe-table \
    --region "$AWS_REGION_FROM_TFVARS" \
    --table-name "$STATE_LOCK_TABLE_NAME" >/dev/null 2>&1; then
    log_note "State lock table exists: $STATE_LOCK_TABLE_NAME"
    return 0
  fi

  log_note "Creating state lock table: $STATE_LOCK_TABLE_NAME"
  aws dynamodb create-table \
    --region "$AWS_REGION_FROM_TFVARS" \
    --table-name "$STATE_LOCK_TABLE_NAME" \
    --attribute-definitions AttributeName=LockID,AttributeType=S \
    --key-schema AttributeName=LockID,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST >/dev/null

  aws dynamodb wait table-exists \
    --region "$AWS_REGION_FROM_TFVARS" \
    --table-name "$STATE_LOCK_TABLE_NAME"
}

check_backend_resources_exist() {
  local bucket_exists=0 table_exists=0
  if aws s3api head-bucket --bucket "$STATE_BUCKET_NAME" >/dev/null 2>&1; then
    bucket_exists=1
  fi
  if aws dynamodb describe-table \
    --region "$AWS_REGION_FROM_TFVARS" \
    --table-name "$STATE_LOCK_TABLE_NAME" >/dev/null 2>&1; then
    table_exists=1
  fi
  if [[ "$bucket_exists" -eq 1 && "$table_exists" -eq 1 ]]; then
    log_note "Terraform backend resources already exist"
    return 0
  fi
  if [[ "$SKIP_BOOTSTRAP" -eq 1 ]]; then
    die "Backend resources missing but --skip-bootstrap was set. Create them first or rerun without --skip-bootstrap."
  fi
  if [[ "$PLAN_ONLY" -eq 1 ]]; then
    die "Plan-only mode: backend resources missing and cannot be created."
  fi

  log_step "Bootstrapping Terraform backend resources (S3 + DynamoDB)"
  confirm_or_exit "Create missing backend resources now?"
  create_backend_bucket_if_missing
  create_backend_lock_table_if_missing
}

run_platform_plan_apply() {
  log_step "Initializing remote backend"
  export AWS_EC2_METADATA_DISABLED=true
  tf init -reconfigure -backend-config="$BACKEND_HCL_FILE"

  log_step "Planning platform infrastructure"
  tf validate
  tf plan -input=false -lock=false -refresh=false

  if [[ "$PLAN_ONLY" -eq 1 ]]; then
    log_note "Plan-only mode enabled; skipping apply."
    return 0
  fi

  log_step "Applying platform infrastructure"
  confirm_or_exit "Apply platform infrastructure changes now?"
  if [[ "$AUTO_APPROVE" -eq 1 ]]; then
    tf apply -input=false -auto-approve
  else
    tf apply -input=false
  fi
}

print_output_or_na() {
  local key="$1"
  local value
  value="$(tf output -raw "$key" 2>/dev/null || true)"
  if [[ -z "$value" || "$value" == "null" ]]; then
    printf '  - %s: %s\n' "$key" "n/a"
  else
    printf '  - %s: %s\n' "$key" "$value"
  fi
}

print_summary() {
  log_step "Important outputs"
  print_output_or_na "backend_service_role_arn"
  print_output_or_na "deploy_runner_role_arn"
  print_output_or_na "github_actions_role_arn"
  print_output_or_na "waf_regional_web_acl_arn"
  print_output_or_na "waf_cloudfront_web_acl_arn"
  print_output_or_na "static_site_url"
  print_output_or_na "control_plane_alb_dns_name"
  print_output_or_na "control_api_db_url"
  print_output_or_na "control_api_db_filesystem_id"
  print_output_or_na "deploy_jobs_queue_url"
  print_output_or_na "webhook_events_queue_url"
  print_output_or_na "ai_runtime_role_arn"

  printf '\nManual steps that remain external:\n'
  printf '  1) Customer AWS onboarding: run CloudFormation template in customer account.\n'
  printf '  2) GitHub setup: app/webhook/branch protections/repo vars.\n'
}

main() {
  parse_args "$@"

  [[ -d "$TF_DIR" ]] || die "Missing Terraform directory: $TF_DIR"

  log_step "Checking prerequisites"
  require_cmd terraform
  require_cmd aws
  require_cmd grep
  require_cmd sed
  require_cmd xargs

  load_config_values
  ensure_backend_hcl
  check_aws_access
  check_backend_resources_exist
  run_platform_plan_apply
  print_summary

  printf '\nDone.\n'
}

main "$@"
