locals {
  s3_bucket_name = "${var.project_name}-website-${var.environment}"
  full_domain    = var.subdomain != "" ? "${var.subdomain}.${var.domain_name}" : var.domain_name
  s3_origin_id   = "S3-${local.s3_bucket_name}"
}

# S3 bucket for website content
resource "aws_s3_bucket" "website" {
  bucket = local.s3_bucket_name

  tags = merge(
    var.tags,
    {
      Name        = local.s3_bucket_name
      Environment = var.environment
    }
  )
}

# S3 bucket versioning
resource "aws_s3_bucket_versioning" "website" {
  bucket = aws_s3_bucket.website.id

  versioning_configuration {
    status = "Enabled"
  }
}

# S3 bucket website configuration
resource "aws_s3_bucket_website_configuration" "website" {
  bucket = aws_s3_bucket.website.id

  index_document {
    suffix = var.default_root_object
  }

  error_document {
    key = var.error_document
  }
}

# S3 bucket public access block
resource "aws_s3_bucket_public_access_block" "website" {
  bucket = aws_s3_bucket.website.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# CloudFront Origin Access Control
resource "aws_cloudfront_origin_access_control" "website" {
  name                              = "${local.s3_bucket_name}-oac"
  description                       = "OAC for ${local.s3_bucket_name}"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# S3 bucket policy to allow CloudFront access
resource "aws_s3_bucket_policy" "website" {
  bucket = aws_s3_bucket.website.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowCloudFrontServicePrincipal"
        Effect = "Allow"
        Principal = {
          Service = "cloudfront.amazonaws.com"
        }
        Action   = "s3:GetObject"
        Resource = "${aws_s3_bucket.website.arn}/*"
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = aws_cloudfront_distribution.website.arn
          }
        }
      }
    ]
  })
}

# CloudFront distribution
resource "aws_cloudfront_distribution" "website" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = "${var.project_name} website - ${var.environment}"
  default_root_object = var.default_root_object
  price_class         = var.price_class
  aliases             = var.enable_ssl && var.domain_name != "" ? [local.full_domain] : []

  origin {
    domain_name              = aws_s3_bucket.website.bucket_regional_domain_name
    origin_id                = local.s3_origin_id
    origin_access_control_id = aws_cloudfront_origin_access_control.website.id
  }

  default_cache_behavior {
    allowed_methods  = ["GET", "HEAD", "OPTIONS"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = local.s3_origin_id

    forwarded_values {
      query_string = false

      cookies {
        forward = "none"
      }
    }

    viewer_protocol_policy = "redirect-to-https"
    min_ttl                = 0
    default_ttl            = 3600
    max_ttl                = 86400
    compress               = true
  }

  # Custom error responses for SPA routing
  custom_error_response {
    error_code         = 404
    response_code      = 200
    response_page_path = "/${var.default_root_object}"
  }

  custom_error_response {
    error_code         = 403
    response_code      = 200
    response_page_path = "/${var.default_root_object}"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = var.enable_ssl ? false : true
    acm_certificate_arn            = var.enable_ssl ? aws_acm_certificate.website[0].arn : null
    ssl_support_method             = var.enable_ssl ? "sni-only" : null
    minimum_protocol_version       = var.enable_ssl ? "TLSv1.2_2021" : "TLSv1"
  }

  tags = merge(
    var.tags,
    {
      Name        = "${var.project_name}-website-cdn"
      Environment = var.environment
    }
  )
}

# ACM Certificate (only if SSL is enabled)
resource "aws_acm_certificate" "website" {
  count = var.enable_ssl && var.domain_name != "" ? 1 : 0

  provider          = aws.us_east_1
  domain_name       = local.full_domain
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = merge(
    var.tags,
    {
      Name        = local.full_domain
      Environment = var.environment
    }
  )
}

# Route53 validation records (only if SSL is enabled)
resource "aws_route53_record" "cert_validation" {
  for_each = var.enable_ssl && var.domain_name != "" ? {
    for dvo in aws_acm_certificate.website[0].domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  } : {}

  allow_overwrite = true
  name            = each.value.name
  records         = [each.value.record]
  ttl             = 60
  type            = each.value.type
  zone_id         = data.aws_route53_zone.main[0].zone_id
}

# ACM certificate validation (only if SSL is enabled)
resource "aws_acm_certificate_validation" "website" {
  count = var.enable_ssl && var.domain_name != "" ? 1 : 0

  provider                = aws.us_east_1
  certificate_arn         = aws_acm_certificate.website[0].arn
  validation_record_fqdns = [for record in aws_route53_record.cert_validation : record.fqdn]
}

# Data source for Route53 zone (only if SSL is enabled)
data "aws_route53_zone" "main" {
  count = var.enable_ssl && var.domain_name != "" ? 1 : 0

  name         = var.domain_name
  private_zone = false
}

# Route53 record for the website (only if SSL is enabled)
resource "aws_route53_record" "website" {
  count = var.enable_ssl && var.domain_name != "" ? 1 : 0

  zone_id = data.aws_route53_zone.main[0].zone_id
  name    = local.full_domain
  type    = "A"

  alias {
    name                   = aws_cloudfront_distribution.website.domain_name
    zone_id                = aws_cloudfront_distribution.website.hosted_zone_id
    evaluate_target_health = false
  }
}

# Route53 AAAA record for IPv6 (only if SSL is enabled)
resource "aws_route53_record" "website_ipv6" {
  count = var.enable_ssl && var.domain_name != "" ? 1 : 0

  zone_id = data.aws_route53_zone.main[0].zone_id
  name    = local.full_domain
  type    = "AAAA"

  alias {
    name                   = aws_cloudfront_distribution.website.domain_name
    zone_id                = aws_cloudfront_distribution.website.hosted_zone_id
    evaluate_target_health = false
  }
}
