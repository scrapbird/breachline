variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "ap-southeast-2"
}

variable "project" {
  description = "Project name for resource tagging"
  type        = string
  default     = "breachline"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "prod"
}

variable "api_domain_name" {
  description = "Custom domain name for the API (e.g., api.breachline.app)"
  type        = string
  default     = "api.breachline.app"
}

variable "ses_email_from" {
  description = "Email address to send PINs from"
  type        = string
  default     = "noreply@breachline.app"
}

variable "ses_verified_domain" {
  description = "Verified domain for SES (e.g., breachline.app)"
  type        = string
  default     = "breachline.app"
}

variable "license_public_key" {
  description = "ECDSA public key for license validation (PEM format)"
  type        = string
  sensitive   = true
}

variable "pin_request_rate_limit" {
  description = "Maximum PIN requests per email per hour"
  type        = number
  default     = 3
}

variable "lambda_memory_size" {
  description = "Memory size for Lambda functions in MB"
  type        = number
  default     = 512
}

variable "lambda_timeout" {
  description = "Timeout for Lambda functions in seconds"
  type        = number
  default     = 30
}

# Change processor timeout variable removed - lambda no longer exists

variable "audit_ttl_days" {
  description = "Number of days to retain audit entries in DynamoDB"
  type        = number
  default     = 90
}

variable "pin_ttl_hours" {
  description = "Number of hours PINs are valid"
  type        = number
  default     = 12
}

# SQS and change processor variables removed - using direct DynamoDB operations instead

variable "alarm_email" {
  description = "Email address for CloudWatch alarms"
  type        = string
}
