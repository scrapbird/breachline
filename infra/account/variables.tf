variable "aws_region" {
  description = "AWS region for the account-wide resources (must match the env stacks)"
  type        = string
  default     = "us-east-2"
}

variable "ses_domain" {
  description = "Domain verified in SES; dev and prod send from addresses on it"
  type        = string
  default     = "breachline.app"
}
