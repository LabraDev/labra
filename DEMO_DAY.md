# Demo Day Runbook (Cloud-First, No Localhost Required)

This runbook is optimized for a professor-facing cloud demo where the app is accessed through AWS URLs instead of `localhost`.

## 1) Goal

Present:

1. Terraform-managed AWS infrastructure is provisioned and healthy.
2. Cloud endpoints are reachable.
3. Remaining manual integration steps (GitHub + customer onboarding CloudFormation) are clearly identified.

## 2) Prerequisites

Install/verify:

```bash
go version
node -v
npm -v
terraform version
aws --version
curl --version
```

Configure AWS:

```bash
aws configure
aws sts get-caller-identity
```

## 3) One-Command Cloud Provisioning + Deployment

Run from repo root:

```bash
./cloud-up.sh --yes
```

What it automates:

1. State backend bootstrap (S3 + DynamoDB) when missing.
2. Remote backend init/re-init.
3. Terraform validate/plan/apply.
4. Backend image build and push to ECR.
5. Cloud API service enablement on ECS/ALB.
6. Frontend build upload to S3 + CloudFront invalidation.
7. Key output summary for demo endpoints and identities.

## 4) Cloud-Only Demo Readiness Check

Run:

```bash
./cloud-demo-checklist.sh --skip-infra --require-api
```

What this script validates:

1. CloudFront endpoint (`static_site_url`) is reachable.
2. API ALB health endpoint is reachable when ALB services are enabled.
3. Persistent DB infra outputs exist (EFS + cloud DB URL).
4. Key infra outputs exist (Cognito, queues, etc.).
5. Remaining manual integrations are printed clearly.

## 5) Cloud URLs to Present

Pull outputs directly:

```bash
terraform -chdir=labra-infra/env/dev output -raw static_site_url
terraform -chdir=labra-infra/env/dev output -raw control_plane_alb_dns_name
terraform -chdir=labra-infra/env/dev output -raw control_api_db_filesystem_id
```

Use:

1. CloudFront URL as frontend URL.
2. ALB DNS URL as API URL (if control-plane services baseline is enabled).
3. EFS filesystem ID as proof DB is fully cloud-backed and persistent.

## 6) Required Manual Steps for Full End-to-End External Integration

These are expected manual actions and are not a Terraform gap:

1. Customer onboarding role creation via CloudFormation template in customer account.
2. GitHub-side setup (app/OAuth/webhook/repo settings/protections/vars).

## 7) DNS/Domain Recommendation for Demo Cost

For demo day, do not buy a domain unless explicitly required.

Recommended:

1. Use CloudFront default URL (already HTTPS).
2. Use ALB DNS URL for API (temporary/demo-grade).
3. No Route53 setup is required for this demo path.

## 8) Terraform Flags for Cloud API and DB Endpoint

In `labra-infra/env/dev/terraform.tfvars`, API cloud endpoint needs:

```hcl
enable_control_plane_services_baseline = true
enable_control_api_db_storage          = true
```

If `enable_control_plane_services_baseline` is false, `control_plane_alb_dns_name` will be empty.

## 9) Presentation Sequence (Cloud-First)

1. Show `cloud-up.sh` success and Terraform outputs.
2. Show `cloud-demo-checklist.sh` success.
3. Open CloudFront URL.
4. Show ALB `/health` if enabled.
5. Explain manual customer CloudFormation onboarding and GitHub integration handoff.

## 10) Localhost Fallback (Only if Cloud Endpoint Is Unavailable)

Fallback command:

```bash
./demo-checklist.sh --aws-validate --keep-running
```

Use only as backup if AWS endpoint verification fails during demo window.
