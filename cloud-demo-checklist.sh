#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TF_DIR="$ROOT_DIR/labra-infra/env/dev"
TFVARS_FILE="$TF_DIR/terraform.tfvars"

RUN_INFRA=1
AUTO_YES=0
REQUIRE_API=0

STEP=1

usage() {
  cat <<'USAGE'
Usage: ./cloud-demo-checklist.sh [options]

Cloud-first demo readiness checklist (no localhost dependency in the demo path).
This script:
1) optionally runs infra-up automation,
2) reads Terraform outputs,
3) verifies cloud endpoints,
4) confirms persistent cloud DB outputs (EFS + DB URL),
5) prints remaining manual integration steps for final end-to-end demo.

Options:
  --skip-infra       Skip running ./infra-up.sh.
  --yes              Pass --yes to infra-up.sh when infra is run.
  --require-api      Fail if API ALB DNS output is missing or API health check fails.
  -h, --help         Show help.
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

tf_out_raw() {
  local key="$1"
  terraform -chdir="$TF_DIR" output -raw "$key" 2>/dev/null || true
}

parse_tfvar_bool() {
  local key="$1"
  local line value
  line="$(grep -E "^[[:space:]]*${key}[[:space:]]*=" "$TFVARS_FILE" | tail -n 1 || true)"
  if [[ -z "$line" ]]; then
    printf 'unknown'
    return 0
  fi
  value="${line#*=}"
  value="$(printf '%s' "$value" | sed -E 's/[[:space:]]+#.*$//' | xargs | tr '[:upper:]' '[:lower:]')"
  case "$value" in
    true|false) printf '%s' "$value" ;;
    *) printf 'unknown' ;;
  esac
}

check_url() {
  local url="$1"
  local label="$2"
  if curl -fsS -m 20 "$url" >/dev/null 2>&1; then
    log_note "${label} reachable: ${url}"
    return 0
  fi
  log_note "${label} not reachable right now: ${url}"
  return 1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --skip-infra)
        RUN_INFRA=0
        ;;
      --yes)
        AUTO_YES=1
        ;;
      --require-api)
        REQUIRE_API=1
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

main() {
  parse_args "$@"

  require_cmd terraform
  require_cmd aws
  require_cmd curl
  require_cmd grep
  require_cmd sed
  require_cmd xargs

  [[ -d "$TF_DIR" ]] || die "Missing Terraform dir: $TF_DIR"
  [[ -f "$TFVARS_FILE" ]] || die "Missing tfvars file: $TFVARS_FILE"

  if [[ "$RUN_INFRA" -eq 1 ]]; then
    log_step "Running Terraform infrastructure automation"
    if [[ "$AUTO_YES" -eq 1 ]]; then
      "$ROOT_DIR/infra-up.sh" --yes
    else
      "$ROOT_DIR/infra-up.sh"
    fi
  else
    log_step "Skipping infra-up execution (--skip-infra)"
  fi

  log_step "Collecting cloud outputs"
  local static_site_url alb_dns cognito_pool cognito_client deploy_q webhook_q db_url db_fs waf_alb waf_cf
  static_site_url="$(tf_out_raw "static_site_url")"
  alb_dns="$(tf_out_raw "control_plane_alb_dns_name")"
  cognito_pool="$(tf_out_raw "cognito_user_pool_id")"
  cognito_client="$(tf_out_raw "cognito_app_client_id")"
  deploy_q="$(tf_out_raw "deploy_jobs_queue_url")"
  webhook_q="$(tf_out_raw "webhook_events_queue_url")"
  db_url="$(tf_out_raw "control_api_db_url")"
  db_fs="$(tf_out_raw "control_api_db_filesystem_id")"
  waf_alb="$(tf_out_raw "waf_regional_web_acl_arn")"
  waf_cf="$(tf_out_raw "waf_cloudfront_web_acl_arn")"

  log_note "CloudFront URL: ${static_site_url:-n/a}"
  log_note "API ALB DNS: ${alb_dns:-n/a}"
  log_note "Cognito User Pool ID: ${cognito_pool:-n/a}"
  log_note "Cognito App Client ID: ${cognito_client:-n/a}"
  log_note "Control API DB URL: ${db_url:-n/a}"
  log_note "Control API EFS ID: ${db_fs:-n/a}"
  log_note "ALB WAF ARN: ${waf_alb:-n/a}"
  log_note "CloudFront WAF ARN: ${waf_cf:-n/a}"
  log_note "Deploy queue URL: ${deploy_q:-n/a}"
  log_note "Webhook queue URL: ${webhook_q:-n/a}"

  log_step "Verifying cloud endpoints"
  if [[ -n "$static_site_url" && "$static_site_url" != "null" ]]; then
    check_url "$static_site_url" "Frontend (CloudFront)" || true
  else
    die "static_site_url output is empty. Infra apply likely incomplete."
  fi

  local enable_services
  enable_services="$(parse_tfvar_bool "enable_control_plane_services_baseline")"
  if [[ -n "$alb_dns" && "$alb_dns" != "null" ]]; then
    if ! check_url "http://${alb_dns}/health" "API /health (ALB)"; then
      if [[ "$REQUIRE_API" -eq 1 ]]; then
        die "API ALB health check failed and --require-api was specified."
      fi
    fi
  else
    log_note "API ALB output is empty."
    if [[ "$enable_services" != "true" ]]; then
      log_note "enable_control_plane_services_baseline is not true in terraform.tfvars."
      log_note "Set it to true, apply infra-up again, and optionally pass --require-api."
    fi
    if [[ "$REQUIRE_API" -eq 1 ]]; then
      die "API ALB is required by --require-api but not present."
    fi
  fi

  log_step "External/manual demo integrations"
  printf '  - Customer onboarding (CloudFormation in customer account): required for real cross-account AssumeRole demo.\n'
  printf '  - GitHub setup (App/OAuth + webhook + branch protections + repo vars): required for real push-to-deploy demo.\n'

  log_step "Cloud demo summary"
  printf '  - Frontend URL (cloud): %s\n' "$static_site_url"
  if [[ -n "$alb_dns" && "$alb_dns" != "null" ]]; then
    printf '  - API URL (cloud): http://%s\n' "$alb_dns"
  else
    printf '  - API URL (cloud): n/a (enable control_plane_services_baseline for ALB-based API endpoint)\n'
  fi

  printf '\nCompleted cloud demo readiness checklist.\n'
}

main "$@"
