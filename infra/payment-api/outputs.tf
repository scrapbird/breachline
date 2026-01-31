# License Generator Outputs
output "license_generator_function_name" {
  description = "Name of the License Generator Lambda function"
  value       = aws_lambda_function.license_generator.function_name
}

output "license_generator_function_arn" {
  description = "ARN of the License Generator Lambda function"
  value       = aws_lambda_function.license_generator.arn
}

output "license_generator_role_arn" {
  description = "ARN of the License Generator Lambda execution role"
  value       = aws_iam_role.license_generator_role.arn
}

output "signing_key_secret_arn" {
  description = "ARN of the Secrets Manager secret containing the signing key"
  value       = aws_secretsmanager_secret.signing_key.arn
}

output "signing_key_secret_name" {
  description = "Name of the Secrets Manager secret containing the signing key"
  value       = aws_secretsmanager_secret.signing_key.name
}

# Stripe Webhook Outputs
output "webhook_url" {
  description = "Primary Stripe webhook endpoint URL (via CloudFront - use this in Stripe dashboard)"
  value       = "https://${aws_cloudfront_distribution.webhook_distribution.domain_name}/webhook"
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID for webhook endpoint"
  value       = aws_cloudfront_distribution.webhook_distribution.id
}

output "cloudfront_domain_name" {
  description = "CloudFront domain name"
  value       = aws_cloudfront_distribution.webhook_distribution.domain_name
}

output "api_gateway_url" {
  description = "Direct API Gateway URL (bypass CloudFront - for testing only)"
  value       = "${aws_api_gateway_stage.stripe_webhook_stage.invoke_url}/webhook"
}

output "stripe_webhook_api_gateway_id" {
  description = "ID of the API Gateway"
  value       = aws_api_gateway_rest_api.stripe_webhook.id
}

output "stripe_webhook_function_name" {
  description = "Name of the Stripe Webhook Lambda function"
  value       = aws_lambda_function.stripe_webhook.function_name
}

output "stripe_webhook_function_arn" {
  description = "ARN of the Stripe Webhook Lambda function"
  value       = aws_lambda_function.stripe_webhook.arn
}

output "stripe_webhook_log_group" {
  description = "CloudWatch Log Group for Stripe Webhook Lambda function logs"
  value       = aws_cloudwatch_log_group.stripe_webhook_logs.name
}

output "license_generator_log_group" {
  description = "CloudWatch Log Group for License Generator Lambda function logs"
  value       = aws_cloudwatch_log_group.license_generator_logs.name
}
