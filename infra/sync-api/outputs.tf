# API Gateway outputs
output "api_gateway_url" {
  description = "Base URL for the API Gateway"
  value       = "${aws_api_gateway_stage.v1.invoke_url}"
}

output "api_gateway_id" {
  description = "ID of the API Gateway"
  value       = aws_api_gateway_rest_api.main.id
}

output "api_gateway_stage" {
  description = "API Gateway stage name"
  value       = aws_api_gateway_stage.v1.stage_name
}

# DynamoDB table outputs
output "dynamodb_tables" {
  description = "Map of DynamoDB table names"
  value = {
    pins              = aws_dynamodb_table.pins.name
    workspaces        = aws_dynamodb_table.workspaces.name
    workspace_members = aws_dynamodb_table.workspace_members.name
    annotations       = aws_dynamodb_table.annotations.name
    audit             = aws_dynamodb_table.audit.name
    workspace_files   = aws_dynamodb_table.workspace_files.name
    file_locations    = aws_dynamodb_table.workspace_file_locations.name
    subscriptions     = aws_dynamodb_table.user_subscriptions.name
  }
}

# Note: SQS queues removed as part of direct DynamoDB operations refactor

# Lambda function outputs
output "lambda_functions" {
  description = "Map of Lambda function ARNs"
  value = {
    for name, func in aws_lambda_function.functions :
    name => func.arn
  }
}

# Note: change_processor lambda removed as part of direct DynamoDB operations refactor

output "authorizer_arn" {
  description = "ARN of the authorizer Lambda"
  value       = aws_lambda_function.authorizer.arn
}

# Secrets Manager outputs
output "jwt_private_key_arn" {
  description = "ARN of the JWT private key in Secrets Manager"
  value       = aws_secretsmanager_secret.jwt_private_key.arn
  sensitive   = true
}

output "jwt_public_key_arn" {
  description = "ARN of the JWT public key in Secrets Manager"
  value       = aws_secretsmanager_secret.jwt_public_key.arn
  sensitive   = true
}

output "license_public_key_arn" {
  description = "ARN of the license public key in Secrets Manager"
  value       = aws_secretsmanager_secret.license_public_key.arn
  sensitive   = true
}

# SES outputs
output "ses_domain_identity" {
  description = "SES domain identity"
  value       = aws_ses_domain_identity.main.domain
}

output "ses_from_email" {
  description = "SES from email address"
  value       = var.ses_email_from
}

output "ses_dkim_tokens" {
  description = "DKIM tokens for DNS configuration"
  value       = aws_ses_domain_dkim.main.dkim_tokens
}

# CloudWatch outputs
output "alarm_topic_arn" {
  description = "ARN of the SNS topic for alarms"
  value       = aws_sns_topic.alarms.arn
}

output "dashboard_name" {
  description = "Name of the CloudWatch dashboard"
  value       = aws_cloudwatch_dashboard.main.dashboard_name
}

# Region output
output "aws_region" {
  description = "AWS region where resources are deployed"
  value       = var.aws_region
}
