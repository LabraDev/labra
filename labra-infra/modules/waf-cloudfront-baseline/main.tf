terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}

resource "aws_wafv2_web_acl" "cloudfront" {
  name        = "${var.name_prefix}-cloudfront-waf"
  scope       = "CLOUDFRONT"
  description = "Baseline AWS managed protections for Labra CloudFront frontend"

  default_action {
    allow {}
  }

  rule {
    name     = "AWSManagedRulesCommonRuleSet"
    priority = 10

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "${replace(var.name_prefix, "-", "")}_cf_common"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${replace(var.name_prefix, "-", "")}_cf_waf"
    sampled_requests_enabled   = true
  }

  tags = merge(var.tags, {
    Component = "waf-cloudfront"
  })
}
