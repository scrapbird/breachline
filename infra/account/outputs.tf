output "ses_domain" {
  description = "SES verified sending domain"
  value       = aws_sesv2_email_identity.domain.email_identity
}

output "ses_dkim_records" {
  description = "CNAME records to add to DNS to verify the SES domain"
  value = [
    for token in aws_sesv2_email_identity.domain.dkim_signing_attributes[0].tokens : {
      type  = "CNAME"
      name  = "${token}._domainkey.${var.ses_domain}"
      value = "${token}.dkim.amazonses.com"
    }
  ]
}

output "api_gateway_cloudwatch_role_arn" {
  description = "ARN of the role API Gateway uses to write CloudWatch logs"
  value       = aws_iam_role.api_gateway_cloudwatch.arn
}
