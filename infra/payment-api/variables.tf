variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "ap-southeast-2"
}

variable "environment" {
  description = "Environment name (dev, prod). Suffixed onto every resource name + tfstate key so dev and prod coexist in one account."
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}

variable "ses_sender_email" {
  description = "From address the license-sender lambda sends licenses from"
  type        = string
  default     = "noreply@breachline.app"
}

variable "stripe_api_key" {
  description = "Stripe secret API key (sk_test_ for dev sandbox, sk_live_ for prod)"
  type        = string
  sensitive   = true
}

variable "stripe_webhook_secret" {
  description = "Stripe webhook signing secret (whsec_...)"
  type        = string
  sensitive   = true
}

variable "license_signing_private_key" {
  description = "ECDSA private key (PEM) used to sign issued licenses"
  type        = string
  sensitive   = true
}
