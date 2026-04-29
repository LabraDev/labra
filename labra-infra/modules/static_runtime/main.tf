data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

data "aws_cloudfront_cache_policy" "caching_optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_cache_policy" "caching_disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_origin_request_policy" "all_viewer" {
  name = "Managed-AllViewer"
}

locals {
  default_bucket_name = substr(
    lower(replace("${var.name_prefix}-${data.aws_caller_identity.current.account_id}-site", "_", "-")),
    0,
    63
  )
  effective_region       = coalesce(var.region, data.aws_region.current.name)
  site_bucket_name       = coalesce(var.bucket_name, local.default_bucket_name)
  origin_id              = "${var.name_prefix}-static-origin"
  api_origin_name        = trimspace(var.api_origin_domain_name == null ? "" : var.api_origin_domain_name)
  api_origin_id          = "${var.name_prefix}-api-origin"
  create_api_origin      = local.api_origin_name != ""
  cloudfront_web_acl_arn = trimspace(var.cloudfront_web_acl_arn == null ? "" : var.cloudfront_web_acl_arn)
  module_tags = merge(var.tags, {
    AppName   = var.app_name
    BuildType = var.build_type
    Region    = local.effective_region
  })
}

resource "aws_s3_bucket" "site" {
  bucket        = local.site_bucket_name
  force_destroy = var.force_destroy

  tags = merge(local.module_tags, {
    Name = local.site_bucket_name
  })
}

resource "aws_s3_bucket_versioning" "site" {
  bucket = aws_s3_bucket.site.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "site" {
  bucket = aws_s3_bucket.site.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "site" {
  bucket = aws_s3_bucket.site.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudfront_origin_access_control" "site" {
  name                              = "${var.name_prefix}-static-oac"
  description                       = "OAC for ${var.name_prefix} static site bucket"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "site" {
  enabled             = true
  default_root_object = var.default_root_object
  price_class         = var.price_class
  wait_for_deployment = false
  web_acl_id          = local.cloudfront_web_acl_arn == "" ? null : local.cloudfront_web_acl_arn

  dynamic "origin" {
    for_each = local.create_api_origin ? [1] : []

    content {
      domain_name         = local.api_origin_name
      origin_id           = local.api_origin_id
      connection_attempts = 3
      connection_timeout  = 10

      custom_origin_config {
        http_port                = 80
        https_port               = 443
        origin_keepalive_timeout = 5
        origin_protocol_policy   = var.api_origin_protocol_policy
        origin_read_timeout      = 30
        origin_ssl_protocols     = ["TLSv1.2"]
      }
    }
  }

  origin {
    domain_name              = aws_s3_bucket.site.bucket_regional_domain_name
    origin_id                = local.origin_id
    origin_access_control_id = aws_cloudfront_origin_access_control.site.id
    connection_attempts      = 3
    connection_timeout       = 10

    s3_origin_config {
      origin_access_identity = ""
    }
  }

  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD", "OPTIONS"]
    cache_policy_id        = data.aws_cloudfront_cache_policy.caching_optimized.id
    target_origin_id       = local.origin_id
    viewer_protocol_policy = "redirect-to-https"
    compress               = true
  }

  dynamic "ordered_cache_behavior" {
    for_each = local.create_api_origin ? [1] : []

    content {
      path_pattern             = var.api_origin_path_pattern
      target_origin_id         = local.api_origin_id
      viewer_protocol_policy   = "https-only"
      allowed_methods          = ["GET", "HEAD", "OPTIONS", "PUT", "PATCH", "POST", "DELETE"]
      cached_methods           = ["GET", "HEAD", "OPTIONS"]
      cache_policy_id          = data.aws_cloudfront_cache_policy.caching_disabled.id
      origin_request_policy_id = data.aws_cloudfront_origin_request_policy.all_viewer.id
      compress                 = false
    }
  }

  dynamic "custom_error_response" {
    for_each = var.enable_spa_routing ? toset([403, 404]) : toset([])

    content {
      error_code            = custom_error_response.value
      response_code         = 200
      response_page_path    = "/index.html"
      error_caching_min_ttl = 60
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  tags = merge(local.module_tags, {
    Name = "${var.name_prefix}-static-cdn"
  })

  lifecycle {
    # Provider/API normalization can reorder origin blocks without semantic changes,
    # causing perpetual in-place diff noise.
    ignore_changes = [origin]
  }
}

data "aws_iam_policy_document" "site_bucket_policy" {
  statement {
    sid    = "DenyInsecureTransport"
    effect = "Deny"
    actions = [
      "s3:*"
    ]
    resources = [
      aws_s3_bucket.site.arn,
      "${aws_s3_bucket.site.arn}/*"
    ]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }

  statement {
    sid    = "AllowCloudFrontReadOnly"
    effect = "Allow"
    actions = [
      "s3:GetObject"
    ]

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    resources = ["${aws_s3_bucket.site.arn}/*"]

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.site.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "site" {
  bucket = aws_s3_bucket.site.id
  policy = data.aws_iam_policy_document.site_bucket_policy.json
}

resource "aws_s3_bucket_lifecycle_configuration" "site" {
  bucket = aws_s3_bucket.site.id

  rule {
    id     = "release-history-retention"
    status = "Enabled"

    filter {
      prefix = var.release_prefix
    }

    expiration {
      days = var.release_retention_days
    }

    noncurrent_version_expiration {
      noncurrent_days = var.noncurrent_retention_days
    }
  }

  rule {
    id     = "abort-incomplete-multipart-uploads"
    status = "Enabled"

    filter {
      prefix = ""
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

resource "aws_cloudwatch_metric_alarm" "cloudfront_5xx_rate" {
  count = var.enable_alarms ? 1 : 0

  alarm_name          = "${var.name_prefix}-static-cf-5xx-rate"
  alarm_description   = "CloudFront 5xx error rate above threshold"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = var.alarm_evaluation_periods
  threshold           = var.cf_5xx_rate_threshold
  metric_name         = "5xxErrorRate"
  namespace           = "AWS/CloudFront"
  period              = var.alarm_period_seconds
  statistic           = "Average"
  treat_missing_data  = "notBreaching"

  dimensions = {
    DistributionId = aws_cloudfront_distribution.site.id
    Region         = "Global"
  }
}
