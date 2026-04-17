# Labra Monorepo

This repository contains the Labra backend API, frontend app, and Terraform
infrastructure with a Terraform-first operational model.

## Project Overview

Labra is organized as a monorepo:

- `labra-backend/`: Go API server, auth/session handlers, deployment flows,
  webhook processing, and backend tests.
- `labra-frontend/`: SvelteKit app, smoke/component tests, and UI build pipeline.
- `labra-infra/`: Terraform modules and environment composition for AWS
  infrastructure plus customer onboarding assets.

## Demo Day From Zero (Recommended)

If you have only coded the project and have not configured anything external yet,
and want a cloud-first demo (no localhost dependency in main flow):

1. Configure AWS CLI credentials:

```bash
aws configure
aws sts get-caller-identity
```

2. Provision/update platform infrastructure:

```bash
./cloud-up.sh --yes
```

3. Run cloud demo readiness checks:

```bash
./cloud-demo-checklist.sh --skip-infra --require-api
```

4. Open cloud endpoints from Terraform outputs:

```bash
terraform -chdir=labra-infra/env/dev output -raw static_site_url
terraform -chdir=labra-infra/env/dev output -raw control_plane_alb_dns_name
terraform -chdir=labra-infra/env/dev output -raw control_api_db_filesystem_id
```

For the complete cloud-first runbook, fallback options, and troubleshooting, see
[DEMO_DAY.md](DEMO_DAY.md).

## Backend Setup Run and Test

From repo root:

```bash
cd labra-backend
cp .env.example .env
```

Minimum local env:

```dotenv
DB_URL=./labra.db
```

Common optional values:

```dotenv
GH_CLIENT_ID=
GH_CLIENT_SECRET=
GITHUB_WEBHOOK_SECRET=
```

Run and validate:

```bash
go run ./cmd
go test ./...
go vet ./...
```

Equivalent Makefile commands:

```bash
make run
make generate
make test
make lint
make test-race
```

## Frontend Setup Run and Test

From repo root:

```bash
cd labra-frontend
npm install
npm run dev
```

Checks and build:

```bash
npm run check
npm run test
npm run build
npm run preview
```

## Infrastructure Terraform Workflows

Active environment composition lives at:

- `labra-infra/env/dev/main.tf`
- `labra-infra/env/dev/variables.tf`
- `labra-infra/env/dev/terraform.tfvars`
- `labra-infra/env/dev/outputs.tf`

Core Terraform workflow:

1. Bootstrap remote state once with `bootstrap_state_backend=true`.
2. Re-initialize with backend config for remote state.
3. Use normal `plan`/`apply` flow with `bootstrap_state_backend=false`.

Core command pattern:

```bash
terraform -chdir=labra-infra/env/dev init -backend=false
AWS_EC2_METADATA_DISABLED=true terraform -chdir=labra-infra/env/dev validate
AWS_EC2_METADATA_DISABLED=true terraform -chdir=labra-infra/env/dev plan -input=false -lock=false -refresh=false
AWS_EC2_METADATA_DISABLED=true terraform -chdir=labra-infra/env/dev apply -input=false
```

One-command baseline infra automation:

```bash
./infra-up.sh --yes
```

This script auto-generates `backend.hcl` (if missing), bootstraps state
resources when needed, initializes remote backend, and runs validate/plan/apply.

One-command full cloud deployment automation (recommended for final demo):

```bash
./cloud-up.sh --yes
```

This script provisions infra, builds and pushes backend images to ECR, enables
cloud API services, deploys frontend assets to S3, invalidates CloudFront, and
verifies cloud endpoints with persistent EFS-backed SQLite storage for the API
and Terraform-managed WAF on both ALB and CloudFront.

For complete Terraform-managed coverage, manual exceptions, and rollout steps,
see [AWS_TERRAFORM_FIRST_PLAN.md](AWS_TERRAFORM_FIRST_PLAN.md).

## Customer Onboarding AssumeRole One-Click

Customer onboarding assets:

- `labra-infra/customer-onboarding/customer-assume-role.cfn.yaml`

Terraform remains the platform IaC model, while customer onboarding is
intentionally CloudFormation-only for the simplest one-click role setup.

CloudFormation flow:

```bash
aws cloudformation deploy \
  --stack-name labra-customer-assume-role \
  --template-file labra-infra/customer-onboarding/customer-assume-role.cfn.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides \
    PlatformPrincipalArn="<LABRA_PLATFORM_ROLE_ARN>" \
    ExternalId="<LABRA_EXTERNAL_ID>" \
    RoleName="LabraCustomerDeployRole"

aws cloudformation describe-stacks \
  --stack-name labra-customer-assume-role \
  --query "Stacks[0].Outputs[?OutputKey=='CustomerRoleArn'].OutputValue" \
  --output text
```

## Phase 4 MVP Runbook (Merged)

Phase 4 behavior:

- GitHub push webhook ingestion and signature verification.
- Duplicate delivery handling.
- Repository + branch routing.
- Auto-triggered deployments with commit metadata.
- Deployment history visibility.

Webhook config summary:

1. In GitHub repo: `Settings -> Webhooks -> Add webhook`.
2. Payload URL: `https://<your-cloudfront-domain>/v1/webhooks/github`.
3. Content type: `application/json`.
4. Secret: exactly matches `GITHUB_WEBHOOK_SECRET`.
5. Events: `Just the push event`.

Cloud replay:

```bash
SECRET='replace-with-long-random-secret'
PAYLOAD='{"ref":"refs/heads/main","after":"abc123def456","repository":{"full_name":"owner/repo"},"head_commit":{"id":"abc123def456","message":"feat: update","author":{"name":"Casey"}}}'
SIG=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')

curl -i https://<your-cloudfront-domain>/v1/webhooks/github \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-GitHub-Delivery: local-replay-1" \
  -H "X-Hub-Signature-256: sha256=$SIG" \
  --data "$PAYLOAD"
```

Verification endpoints:

- `GET /v1/apps/:id/deploys`
- `GET /v1/deploys/:id`
- `GET /v1/deploys/:id/logs`

## Phase 7 AI Runbook (Merged)

Endpoints:

- `POST /v1/ai/deploy-insights`
- `GET /v1/ai/requests`

Required input:

- Authenticated user context (`Authorization` or `X-User-ID` locally).
- `deployment_id` in request payload.

Feature flags:

- `AI_FEATURE_ENABLED` (default `true`)
- `AI_KILL_SWITCH_ENABLED` (default `false`)
- `AI_PROMPT_VERSION` (default `phase7-v1`)
- `AI_PROVIDER_MODEL` (default `mock-ops-v1`)

Safety controls:

- Prompt redaction for tokens, API keys, and emails.
- Provider timeout/retry guardrails.
- Fallback insight path on provider failure or kill-switch.
- Audit event emission for AI requests.

Example request:

```bash
curl -sS -X POST https://<your-cloudfront-domain>/v1/ai/deploy-insights \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <SESSION_TOKEN>' \
  -d '{"deployment_id": 10, "prompt": "why did deploy fail?", "bypass_ai": false}'
```

## Phase 8 Readiness Checklist (Merged)

Readiness endpoint:

- `GET /v1/system/readiness-checklist`

Checklist intent:

- Verify webhook replay guardrails.
- Verify webhook secret configuration.
- Verify AI prompt versioning and timeout/fallback controls.
- Verify service inventory includes AI component.

Operational review commands:

```bash
cd labra-backend && go test ./...
cd labra-frontend && npm run check && node --test tests/*.test.mjs
AWS_EC2_METADATA_DISABLED=true terraform -chdir=labra-infra/env/dev validate
curl -sS https://<your-cloudfront-domain>/v1/system/readiness-checklist -H 'Authorization: Bearer <SESSION_TOKEN>'
```

## Notes

- Backlog note from prior TODO:
  use a Nix flake to pin dependencies and manage environment consistency.
