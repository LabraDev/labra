roadmap_phase   = "Phase 8"
roadmap_version = "Ver 1.0"
cost_center     = "cpsc465"

enable_foundation_modules      = true
vpc_enable_nat_gateway         = false
enable_control_plane_cluster   = true
enable_ecr_baseline            = true
enable_waf_regional_baseline   = true
enable_waf_cloudfront_baseline = true

# Optional: enable these to have Terraform fully manage runtime/control-plane resources.
# enable_control_plane_services_baseline = true
# enable_cloudtrail_baseline             = true
# enable_control_api_db_storage          = true

# Optional: allow Terraform to create/manage the GitHub OIDC provider for CI/CD.
# iam_create_github_oidc_provider = true

# If OIDC provider is managed outside Terraform, provide its ARN and enable role.
iam_enable_github_oidc_role = false
# iam_github_oidc_provider_arn = "arn:aws:iam::<account-id>:oidc-provider/token.actions.githubusercontent.com"
# iam_github_repository = "owner/repo"
