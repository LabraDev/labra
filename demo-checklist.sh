#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/labra-backend"
FRONTEND_DIR="$ROOT_DIR/labra-frontend"
INFRA_DIR="$ROOT_DIR/labra-infra"
BACKEND_ENV="$BACKEND_DIR/.env"
BACKEND_ENV_EXAMPLE="$BACKEND_DIR/.env.example"
CFN_TEMPLATE="$INFRA_DIR/customer-onboarding/customer-assume-role.cfn.yaml"

RUN_AWS_VALIDATE=0
SKIP_INSTALL=0
SKIP_TESTS=0
SKIP_INFRA=0
KEEP_RUNNING=0

BACKEND_PID=""
FRONTEND_PID=""
BACKEND_LOG="/tmp/labra-backend-demo.log"
FRONTEND_LOG="/tmp/labra-frontend-demo.log"

STEP=1

usage() {
  cat <<'USAGE'
Usage: ./demo-checklist.sh [options]

Automates local demo readiness for the Labra monorepo:
- env bootstrap for backend
- backend/frontend/infra checks
- backend + frontend startup
- auth, app, deploy, AI, and webhook smoke flow

Options:
  --aws-validate      Run aws cloudformation validate-template.
  --skip-install      Skip npm install.
  --skip-tests        Skip backend/frontend test and check commands.
  --skip-infra        Skip terraform fmt/init/validate.
  --keep-running      Keep started backend/frontend processes alive after script exits.
  -h, --help          Show this help.
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

cleanup() {
  if [[ "$KEEP_RUNNING" -eq 1 ]]; then
    return
  fi

  if [[ -n "$BACKEND_PID" ]] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
    kill "$BACKEND_PID" >/dev/null 2>&1 || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi

  if [[ -n "$FRONTEND_PID" ]] && kill -0 "$FRONTEND_PID" >/dev/null 2>&1; then
    kill "$FRONTEND_PID" >/dev/null 2>&1 || true
    wait "$FRONTEND_PID" 2>/dev/null || true
  fi
}

tail_logs_on_error() {
  local exit_code="$?"
  if [[ "$exit_code" -ne 0 ]]; then
    printf '\nCommand failed. Recent service logs:\n'
    if [[ -f "$BACKEND_LOG" ]]; then
      printf '\n--- backend log (tail) ---\n'
      tail -n 30 "$BACKEND_LOG" || true
    fi
    if [[ -f "$FRONTEND_LOG" ]]; then
      printf '\n--- frontend log (tail) ---\n'
      tail -n 30 "$FRONTEND_LOG" || true
    fi
  fi
  exit "$exit_code"
}

add_env_if_missing() {
  local key="$1"
  local value="$2"
  if ! grep -Eq "^${key}=" "$BACKEND_ENV"; then
    printf '%s=%s\n' "$key" "$value" >> "$BACKEND_ENV"
  fi
}

wait_for_url() {
  local url="$1"
  local timeout_seconds="$2"
  local i
  for ((i = 1; i <= timeout_seconds; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

run() {
  local cmd="$1"
  log_note "$cmd"
  bash -lc "$cmd"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --aws-validate)
        RUN_AWS_VALIDATE=1
        ;;
      --skip-install)
        SKIP_INSTALL=1
        ;;
      --skip-tests)
        SKIP_TESTS=1
        ;;
      --skip-infra)
        SKIP_INFRA=1
        ;;
      --keep-running)
        KEEP_RUNNING=1
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

validate_layout() {
  [[ -d "$BACKEND_DIR" ]] || die "Missing directory: $BACKEND_DIR"
  [[ -d "$FRONTEND_DIR" ]] || die "Missing directory: $FRONTEND_DIR"
  [[ -d "$INFRA_DIR" ]] || die "Missing directory: $INFRA_DIR"
  [[ -f "$CFN_TEMPLATE" ]] || die "Missing CloudFormation template: $CFN_TEMPLATE"
}

ensure_dependencies() {
  log_step "Checking local toolchain"
  require_cmd go
  require_cmd node
  require_cmd npm
  require_cmd terraform
  require_cmd curl
  require_cmd jq
  require_cmd openssl

  if [[ "$RUN_AWS_VALIDATE" -eq 1 ]]; then
    require_cmd aws
  fi

  log_note "go: $(go version)"
  log_note "node: $(node -v)"
  log_note "npm: $(npm -v)"
  log_note "terraform: $(terraform version | head -n 1)"
  if command -v aws >/dev/null 2>&1; then
    log_note "aws: $(aws --version 2>&1)"
  fi
}

ensure_backend_env() {
  log_step "Preparing backend environment file"
  if [[ ! -f "$BACKEND_ENV" ]]; then
    [[ -f "$BACKEND_ENV_EXAMPLE" ]] || die "Missing env example: $BACKEND_ENV_EXAMPLE"
    cp "$BACKEND_ENV_EXAMPLE" "$BACKEND_ENV"
    log_note "Created $BACKEND_ENV from .env.example"
  fi

  local jwt_secret webhook_secret
  jwt_secret="$(openssl rand -hex 24)"
  webhook_secret="$(openssl rand -hex 24)"

  add_env_if_missing "APP_ENV" "dev"
  add_env_if_missing "API_HOST" "localhost"
  add_env_if_missing "API_PORT" "8080"
  add_env_if_missing "JWT_ISSUER" "labra-local-issuer"
  add_env_if_missing "JWT_AUDIENCE" "labra-local-audience"
  add_env_if_missing "JWT_SIGNING_SECRET" "$jwt_secret"
  add_env_if_missing "GITHUB_WEBHOOK_SECRET" "$webhook_secret"
  add_env_if_missing "AI_FEATURE_ENABLED" "true"
  add_env_if_missing "AI_KILL_SWITCH_ENABLED" "false"
  add_env_if_missing "AI_PROMPT_VERSION" "phase7-v1"
  add_env_if_missing "AI_PROVIDER_MODEL" "mock-ops-v1"
  add_env_if_missing "AI_PROVIDER_TIMEOUT_MS" "1800"
  add_env_if_missing "AI_PROVIDER_RETRIES" "2"

  log_note "Ensured required values exist in $BACKEND_ENV"
}

run_quality_checks() {
  if [[ "$SKIP_INSTALL" -eq 0 ]]; then
    log_step "Installing frontend dependencies"
    run "cd '$FRONTEND_DIR' && npm install"
  else
    log_step "Skipping npm install (--skip-install)"
  fi

  if [[ "$SKIP_TESTS" -eq 0 ]]; then
    log_step "Running backend and frontend validation"
    run "cd '$BACKEND_DIR' && go test ./..."
    run "cd '$BACKEND_DIR' && go vet ./..."
    run "cd '$FRONTEND_DIR' && npm run check"
    run "cd '$FRONTEND_DIR' && npm run test"
    run "cd '$FRONTEND_DIR' && npm run build"
  else
    log_step "Skipping backend/frontend tests (--skip-tests)"
  fi

  if [[ "$SKIP_INFRA" -eq 0 ]]; then
    log_step "Running Terraform validation"
    run "cd '$ROOT_DIR' && terraform -chdir=labra-infra/env/dev fmt -check -recursive ../.."
    run "cd '$ROOT_DIR' && terraform -chdir=labra-infra/env/dev init -backend=false"
    run "cd '$ROOT_DIR' && terraform -chdir=labra-infra/env/dev validate"
  else
    log_step "Skipping Terraform checks (--skip-infra)"
  fi

  if [[ "$RUN_AWS_VALIDATE" -eq 1 ]]; then
    log_step "Validating CloudFormation template with AWS API"
    run "cd '$ROOT_DIR' && aws cloudformation validate-template --template-body file://labra-infra/customer-onboarding/customer-assume-role.cfn.yaml >/dev/null"
  else
    log_step "Skipping AWS CloudFormation API validation (use --aws-validate to enable)"
  fi
}

start_services() {
  log_step "Starting local backend/frontend services"

  if curl -fsS "http://localhost:8080/health" >/dev/null 2>&1; then
    log_note "Backend already running on :8080"
  else
    run "cd '$BACKEND_DIR' && nohup go run ./cmd > '$BACKEND_LOG' 2>&1 & echo \$! > '$ROOT_DIR/.backend_demo.pid'"
    BACKEND_PID="$(cat "$ROOT_DIR/.backend_demo.pid")"
    rm -f "$ROOT_DIR/.backend_demo.pid"
    log_note "Started backend PID $BACKEND_PID"
  fi

  if curl -fsS "http://localhost:5173" >/dev/null 2>&1; then
    log_note "Frontend already running on :5173"
  else
    run "cd '$FRONTEND_DIR' && nohup npm run dev -- --host 127.0.0.1 --port 5173 > '$FRONTEND_LOG' 2>&1 & echo \$! > '$ROOT_DIR/.frontend_demo.pid'"
    FRONTEND_PID="$(cat "$ROOT_DIR/.frontend_demo.pid")"
    rm -f "$ROOT_DIR/.frontend_demo.pid"
    log_note "Started frontend PID $FRONTEND_PID"
  fi

  wait_for_url "http://localhost:8080/health" 60 || die "Backend did not become healthy on :8080"
  wait_for_url "http://localhost:8080/ready" 60 || die "Backend did not become ready on :8080"
  wait_for_url "http://localhost:5173" 60 || die "Frontend did not become available on :5173"

  log_note "Backend and frontend are reachable"
}

extract_git_repo_name() {
  local remote_url
  remote_url="$(git -C "$ROOT_DIR" remote get-url origin 2>/dev/null || true)"
  remote_url="${remote_url#git@github.com:}"
  remote_url="${remote_url#https://github.com/}"
  remote_url="${remote_url%.git}"
  if [[ "$remote_url" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    printf '%s' "$remote_url"
  else
    printf 'owner/repo'
  fi
}

run_smoke_flow() {
  log_step "Running end-to-end API smoke flow"

  set -a
  # shellcheck disable=SC1090
  source "$BACKEND_ENV"
  set +a

  [[ -n "${JWT_SIGNING_SECRET:-}" ]] || die "JWT_SIGNING_SECRET missing in $BACKEND_ENV"
  [[ -n "${JWT_ISSUER:-}" ]] || die "JWT_ISSUER missing in $BACKEND_ENV"
  [[ -n "${JWT_AUDIENCE:-}" ]] || die "JWT_AUDIENCE missing in $BACKEND_ENV"
  [[ -n "${GITHUB_WEBHOOK_SECRET:-}" ]] || die "GITHUB_WEBHOOK_SECRET missing in $BACKEND_ENV"

  local external_jwt session_token profile_json user_id repo_full_name
  local timestamp app_name app_branch app_payload app_json app_id
  local deploy_json deploy_id ai_json payload signature webhook_json history_json

  external_jwt="$(node - <<'NODE'
const crypto = require('crypto');
const now = Math.floor(Date.now() / 1000);
const header = { alg: 'HS256', typ: 'JWT' };
const payload = {
  iss: process.env.JWT_ISSUER,
  aud: process.env.JWT_AUDIENCE,
  sub: 'demo-user-001',
  email: 'demo@labra.dev',
  roles: ['owner'],
  exp: now + 3600
};
const b64 = (v) => Buffer.from(JSON.stringify(v)).toString('base64url');
const unsigned = `${b64(header)}.${b64(payload)}`;
const sig = crypto.createHmac('sha256', process.env.JWT_SIGNING_SECRET).update(unsigned).digest('base64url');
process.stdout.write(`${unsigned}.${sig}`);
NODE
)"

  session_token="$(curl -fsS -X POST "http://localhost:8080/v1/auth/session" \
    -H "Authorization: Bearer ${external_jwt}" | jq -er '.session.token')"
  log_note "Created demo auth session token"

  profile_json="$(curl -fsS "http://localhost:8080/v1/profile" \
    -H "Authorization: Bearer ${session_token}")"
  user_id="$(printf '%s' "$profile_json" | jq -er '.user.id')"
  log_note "Profile resolved for user_id=${user_id}"

  repo_full_name="$(extract_git_repo_name)"
  timestamp="$(date +%s)"
  app_name="Demo App ${timestamp}"
  app_branch="demo-${timestamp}"

  app_payload="$(jq -nc \
    --arg name "$app_name" \
    --arg repo "$repo_full_name" \
    --arg branch "$app_branch" \
    '{name:$name,repo_full_name:$repo,branch:$branch,build_type:"static",output_dir:"dist",auto_deploy_enabled:true}')"

  app_json="$(curl -fsS -X POST "http://localhost:8080/v1/apps" \
    -H "Authorization: Bearer ${session_token}" \
    -H "Content-Type: application/json" \
    --data "$app_payload")"
  app_id="$(printf '%s' "$app_json" | jq -er '.id')"
  log_note "Created app id=${app_id} repo=${repo_full_name} branch=${app_branch}"

  deploy_json="$(curl -fsS -X POST "http://localhost:8080/v1/apps/${app_id}/deploy" \
    -H "Authorization: Bearer ${session_token}" \
    -H "Content-Type: application/json" \
    --data '{}')"
  deploy_id="$(printf '%s' "$deploy_json" | jq -er '.deployment.id')"
  log_note "Triggered deployment id=${deploy_id}"

  sleep 2
  curl -fsS "http://localhost:8080/v1/deploys/${deploy_id}" \
    -H "Authorization: Bearer ${session_token}" >/dev/null
  curl -fsS "http://localhost:8080/v1/deploys/${deploy_id}/logs" \
    -H "Authorization: Bearer ${session_token}" >/dev/null
  log_note "Deployment status and logs endpoints reachable"

  ai_json="$(jq -nc \
    --argjson deployment_id "$deploy_id" \
    --arg prompt "why did deploy fail?" \
    '{deployment_id:$deployment_id,prompt:$prompt,bypass_ai:false}')"
  curl -fsS -X POST "http://localhost:8080/v1/ai/deploy-insights" \
    -H "Authorization: Bearer ${session_token}" \
    -H "Content-Type: application/json" \
    --data "$ai_json" >/dev/null
  log_note "AI insight endpoint reachable"

  payload="$(jq -nc \
    --arg ref "refs/heads/${app_branch}" \
    --arg after "abc123def456" \
    --arg repo "$repo_full_name" \
    '{ref:$ref,after:$after,repository:{full_name:$repo},head_commit:{id:$after,message:"demo webhook push",author:{name:"Labra Demo"}}}')"
  signature="$(printf '%s' "$payload" | openssl dgst -sha256 -hmac "$GITHUB_WEBHOOK_SECRET" | sed 's/^.* //')"

  webhook_json="$(curl -fsS -X POST "http://localhost:8080/v1/webhooks/github" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: push" \
    -H "X-GitHub-Delivery: demo-delivery-${timestamp}" \
    -H "X-Hub-Signature-256: sha256=${signature}" \
    --data "$payload")"
  log_note "Webhook replay accepted (delivery demo-delivery-${timestamp})"

  history_json="$(curl -fsS "http://localhost:8080/v1/apps/${app_id}/deploys" \
    -H "Authorization: Bearer ${session_token}")"
  log_note "Deployment history count: $(printf '%s' "$history_json" | jq -r '.deployments | length')"

  printf '\nDemo Smoke Summary\n'
  printf '  app_id: %s\n' "$app_id"
  printf '  deploy_id: %s\n' "$deploy_id"
  printf '  repo_full_name: %s\n' "$repo_full_name"
  printf '  branch: %s\n' "$app_branch"
  printf '  webhook_triggered_count: %s\n' "$(printf '%s' "$webhook_json" | jq -r '.triggered_count')"
  printf '  frontend_url: http://localhost:5173\n'
  printf '  backend_health: http://localhost:8080/health\n'
}

main() {
  trap tail_logs_on_error ERR
  trap cleanup EXIT

  parse_args "$@"
  validate_layout
  ensure_dependencies
  ensure_backend_env
  run_quality_checks
  start_services
  run_smoke_flow

  if [[ "$KEEP_RUNNING" -eq 1 ]]; then
    printf '\nServices are still running because --keep-running was used.\n'
    printf 'Backend log: %s\n' "$BACKEND_LOG"
    printf 'Frontend log: %s\n' "$FRONTEND_LOG"
  else
    printf '\nCompleted. Services started by this script were stopped.\n'
  fi
}

main "$@"
