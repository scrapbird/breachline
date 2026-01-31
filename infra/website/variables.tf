variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "ap-southeast-2"
}

variable "domain_name" {
  description = "Domain name for the website (e.g., example.com)"
  type        = string
  default     = ""
}

variable "subdomain" {
  description = "Subdomain for the website (e.g., www)"
  type        = string
  default     = ""
}

variable "environment" {
  description = "Environment name (e.g., production, staging)"
  type        = string
  default     = "production"
}

variable "project_name" {
  description = "Project name for resource naming"
  type        = string
  default     = "breachline"
}

variable "enable_ssl" {
  description = "Enable SSL/TLS certificate for custom domain"
  type        = bool
  default     = false
}

variable "price_class" {
  description = "CloudFront distribution price class"
  type        = string
  default     = "PriceClass_100"
}

variable "default_root_object" {
  description = "Default root object for the website"
  type        = string
  default     = "index.html"
}

variable "error_document" {
  description = "Error document for the website"
  type        = string
  default     = "error.html"
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default = {
    project   = "breachline"
    managed_by = "terraform"
  }
}
