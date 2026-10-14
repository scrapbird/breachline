variable "aws_region" {
  description = "AWS region of the payment-api stack (where the webhook secret lives)"
  type        = string
  default     = "us-east-2"
}

variable "environment" {
  description = "Deploy environment (dev or prod); selects the payment-api state to read"
  type        = string

  validation {
    condition     = contains(["dev", "prod"], var.environment)
    error_message = "environment must be dev or prod."
  }
}

variable "stripe_api_key" {
  description = "Stripe secret API key (sk_test_ for the dev sandbox, sk_live_ for prod)"
  type        = string
  sensitive   = true
}

variable "success_url" {
  description = "Where payment links send the customer after checkout. Empty uses Stripe's hosted confirmation page."
  type        = string
  default     = ""
}

variable "webhook_api_version" {
  description = "Stripe API version webhook events are rendered in. Must match the stripe-go major version used by payment-api's stripe-webhook Lambda."
  type        = string
  default     = "2024-04-10" # stripe-go v78
}
