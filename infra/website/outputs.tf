output "s3_bucket_name" {
  description = "Name of the S3 bucket"
  value       = aws_s3_bucket.website.id
}

output "s3_bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.website.arn
}

output "s3_bucket_regional_domain_name" {
  description = "Regional domain name of the S3 bucket"
  value       = aws_s3_bucket.website.bucket_regional_domain_name
}

output "cloudfront_distribution_id" {
  description = "ID of the CloudFront distribution"
  value       = aws_cloudfront_distribution.website.id
}

output "cloudfront_distribution_arn" {
  description = "ARN of the CloudFront distribution"
  value       = aws_cloudfront_distribution.website.arn
}

output "cloudfront_domain_name" {
  description = "Domain name of the CloudFront distribution"
  value       = aws_cloudfront_distribution.website.domain_name
}

output "website_url" {
  description = "URL of the website"
  value       = var.enable_ssl && var.domain_name != "" ? "https://${local.full_domain}" : "https://${aws_cloudfront_distribution.website.domain_name}"
}

output "acm_certificate_arn" {
  description = "ARN of the ACM certificate (if SSL is enabled)"
  value       = var.enable_ssl && var.domain_name != "" ? aws_acm_certificate.website[0].arn : null
}

output "route53_name_servers" {
  description = "Name servers for the Route53 zone (if SSL is enabled)"
  value       = var.enable_ssl && var.domain_name != "" ? data.aws_route53_zone.main[0].name_servers : null
}
