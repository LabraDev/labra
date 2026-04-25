resource "aws_wafv2_web_acl" "regional" {
  name        = "${var.name_prefix}-regional-waf"
  scope       = "REGIONAL"
  description = "Baseline AWS managed protections for Labra regional entrypoints"

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
      metric_name                = "${replace(var.name_prefix, "-", "")}_common"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${replace(var.name_prefix, "-", "")}_waf"
    sampled_requests_enabled   = true
  }

  tags = merge(var.tags, {
    Component = "waf-regional"
  })
}

resource "aws_wafv2_web_acl_association" "regional" {
  resource_arn = var.associate_resource_arn
  web_acl_arn  = aws_wafv2_web_acl.regional.arn
}
