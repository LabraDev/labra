# AWS Terraform-First Plan

This document defines the default deployment model for Labra: **cloud-first, Terraform-managed platform infrastructure**, with only unavoidable external setup left manual.

## Terraform-managed AWS acquisition

### Foundation and networking
- Terraform state backend bootstrap: `S3 + DynamoDB`.
- VPC baseline: public/private subnets, route tables, internet gateway, optional NAT.
- Security groups: frontend ingress, API ingress, internal east-west.
- KMS baseline for encryption.
- CloudWatch log groups + metric alarms.
- Secrets Manager placeholder secret.

### Identity and access
- IAM roles/policies for backend service, deploy-runner, and optional GitHub OIDC deploy role.
- Optional Terraform-managed GitHub OIDC provider.
- Cognito user pool, app client, and hosted domain.

### Runtime and delivery
- ECR repositories for deployable service images.
- ECS control-plane cluster.
- ECS/ALB control-plane services (when enabled):
  - `control-api` service behind ALB.
  - `deploy-orchestrator` worker service.
  - `webhook-ingestor` worker service.
- CloudFront + S3 static runtime for frontend.
- SQS deploy/webhook queues + DLQs.

### Persistent cloud DB (default)
- EFS file system + mount targets for `control-api` SQLite data.
- ECS task definition mounts EFS at `/mnt/labra-db`.
- `DB_URL` resolves to `/mnt/labra-db/labra.db` in cloud mode.
- Result: DB writes persist across ECS task restarts/redeploys.

### AI and hardening
- AI runtime IAM role + CloudWatch log group.
- AI feature/kill-switch SSM parameters.
- Optional CloudTrail baseline.
- Regional WAF baseline on ALB.
- CloudFront-scope WAF baseline on frontend distribution.

## Current AWS resources (subject to change)
- State backend:
  - `aws_s3_bucket` (Terraform state)
  - `aws_dynamodb_table` (Terraform lock table)
- Networking:
  - `aws_vpc`
  - `aws_subnet` (public/private)
  - `aws_route_table`, `aws_route_table_association`
  - `aws_internet_gateway`
  - optional `aws_nat_gateway`
- Security:
  - `aws_security_group` (frontend/API/internal)
- IAM/Auth:
  - `aws_iam_role`, `aws_iam_policy`, `aws_iam_role_policy_attachment`
  - optional `aws_iam_openid_connect_provider` (GitHub)
  - `aws_cognito_user_pool`, `aws_cognito_user_pool_client`, optional `aws_cognito_user_pool_domain`
- Observability:
  - `aws_cloudwatch_log_group`, `aws_cloudwatch_metric_alarm`
- Secrets/crypto:
  - `aws_kms_key`, `aws_kms_alias`
  - `aws_secretsmanager_secret`
- Runtime:
  - `aws_ecr_repository`, `aws_ecr_lifecycle_policy`
  - `aws_ecs_cluster`, `aws_ecs_task_definition`, `aws_ecs_service`
  - `aws_lb`, `aws_lb_listener`, `aws_lb_target_group`
  - `aws_service_discovery_private_dns_namespace`
- Frontend delivery:
  - `aws_s3_bucket` (static site)
  - `aws_cloudfront_distribution`
  - OAC + bucket policy + lifecycle config
- Messaging:
  - `aws_sqs_queue` (deploy jobs, webhook events, and DLQs)
- Persistent DB storage:
  - `aws_efs_file_system`
  - `aws_efs_mount_target`
- Optional governance/hardening:
  - `aws_cloudtrail` (+ trail bucket/policy)
  - `aws_wafv2_web_acl` (regional + cloudfront scope)

## Manual or external steps that remain
These are intentionally outside Terraform-only platform control:

1. **Customer account onboarding**
- Customer runs CloudFormation template:
  - `labra-infra/customer-onboarding/customer-assume-role.cfn.yaml`
- Customer returns created role ARN.

2. **GitHub-side integration**
- Configure GitHub OAuth/App credentials.
- Configure webhook + secret.
- Configure repo vars/protections as needed.

## Customer one-click onboarding artifact
- `labra-infra/customer-onboarding/customer-assume-role.cfn.yaml`

This creates the customer-side IAM role + trust policy with `ExternalId` guardrail.

## Recommended `terraform.tfvars` rollout

1. Baseline:
- `enable_foundation_modules=true`
- `enable_cognito_baseline=true`
- `enable_control_plane_cluster=true`
- `enable_ecr_baseline=true`

2. Full cloud runtime:
- `enable_control_plane_services_baseline=true`
- `enable_control_api_db_storage=true`

3. Optional hardening:
- `enable_cloudtrail_baseline=true`
- `enable_waf_regional_baseline=true`
- `enable_waf_cloudfront_baseline=true`

4. Optional OIDC automation:
- `iam_create_github_oidc_provider=true`
- `iam_enable_github_oidc_role=true`
- `iam_github_repository=<owner/repo>`

## Detailed go-live checklist (repo-only starting point)

1. Configure AWS CLI credentials.
2. Run baseline + cloud deployment automation:
- `./cloud-up.sh --yes`

This handles:
- backend state bootstrap (if missing)
- Terraform init/validate/plan/apply
- ECR login + backend image build/push
- ECS service enablement
- EFS-backed DB runtime wiring
- frontend S3 upload + CloudFront invalidation

3. Validate cloud endpoints and outputs:
- `./cloud-demo-checklist.sh --skip-infra --require-api`

4. Capture required outputs:
- `terraform -chdir=labra-infra/env/dev output -raw static_site_url`
- `terraform -chdir=labra-infra/env/dev output -raw control_plane_alb_dns_name`
- `terraform -chdir=labra-infra/env/dev output -raw control_api_db_filesystem_id`

5. Complete manual external integrations:
- Customer CloudFormation onboarding.
- GitHub OAuth/App + webhook wiring.

After these, the demo path is cloud-only (no localhost requirement).
