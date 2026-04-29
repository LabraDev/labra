data "aws_iam_policy_document" "ecs_task_assume_role" {
  statement {
    effect = "Allow"
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
    actions = ["sts:AssumeRole"]
  }
}

locals {
  github_oidc_provider_arn_effective = var.create_github_oidc_provider ? aws_iam_openid_connect_provider.github_actions[0].arn : var.github_oidc_provider_arn
}

resource "aws_iam_role" "backend_service" {
  name               = "${var.name_prefix}-backend-service-role"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume_role.json
  tags               = var.tags
}

resource "aws_iam_role" "deploy_runner" {
  name               = "${var.name_prefix}-deploy-runner-role"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_assume_role.json
  tags               = var.tags
}

data "aws_iam_policy_document" "backend_policy" {
  statement {
    effect = "Allow"
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
      "sts:AssumeRole",
      "bedrock:InvokeModel",
      "bedrock:InvokeModelWithResponseStream"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "backend_policy" {
  name   = "${var.name_prefix}-backend-policy"
  policy = data.aws_iam_policy_document.backend_policy.json
  tags   = var.tags
}

resource "aws_iam_role_policy_attachment" "backend_policy" {
  role       = aws_iam_role.backend_service.name
  policy_arn = aws_iam_policy.backend_policy.arn
}

data "aws_iam_policy_document" "deploy_runner_policy" {
  statement {
    effect = "Allow"
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
      "ecr:GetAuthorizationToken",
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
      "sts:AssumeRole"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "deploy_runner_policy" {
  name   = "${var.name_prefix}-deploy-runner-policy"
  policy = data.aws_iam_policy_document.deploy_runner_policy.json
  tags   = var.tags
}

resource "aws_iam_role_policy_attachment" "deploy_runner_policy" {
  role       = aws_iam_role.deploy_runner.name
  policy_arn = aws_iam_policy.deploy_runner_policy.arn
}

resource "aws_iam_openid_connect_provider" "github_actions" {
  count = var.create_github_oidc_provider ? 1 : 0

  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = var.github_oidc_client_ids
  thumbprint_list = var.github_oidc_thumbprints

  tags = merge(var.tags, {
    Component = "github-oidc-provider"
  })
}

data "aws_iam_policy_document" "github_actions_assume_role" {
  count = var.enable_github_oidc_role ? 1 : 0

  statement {
    effect = "Allow"

    principals {
      type        = "Federated"
      identifiers = [local.github_oidc_provider_arn_effective]
    }

    actions = ["sts:AssumeRoleWithWebIdentity"]

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["repo:${var.github_repository}:*"]
    }
  }
}

resource "aws_iam_role" "github_actions" {
  count              = var.enable_github_oidc_role ? 1 : 0
  name               = "${var.name_prefix}-github-actions-role"
  assume_role_policy = data.aws_iam_policy_document.github_actions_assume_role[0].json
  tags               = var.tags
}

data "aws_iam_policy_document" "github_actions_policy" {
  count = var.enable_github_oidc_role ? 1 : 0

  statement {
    effect = "Allow"
    actions = [
      "sts:GetCallerIdentity",
      "ecs:DescribeClusters",
      "ecs:DescribeServices",
      "ecs:UpdateService",
      "cloudfront:CreateInvalidation",
      "s3:ListBucket",
      "s3:GetObject",
      "s3:PutObject",
      "logs:DescribeLogGroups",
      "iam:PassRole"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "github_actions_policy" {
  count  = var.enable_github_oidc_role ? 1 : 0
  name   = "${var.name_prefix}-github-actions-policy"
  policy = data.aws_iam_policy_document.github_actions_policy[0].json
  tags   = var.tags
}

resource "aws_iam_role_policy_attachment" "github_actions_policy" {
  count      = var.enable_github_oidc_role ? 1 : 0
  role       = aws_iam_role.github_actions[0].name
  policy_arn = aws_iam_policy.github_actions_policy[0].arn
}
