terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      project     = "breachline"
      component   = "sync-api"
      managed_by  = "terraform"
      environment = var.environment
    }
  }
}

locals {
  # Appended to every globally-scoped resource name so dev + prod never
  # collide in the same AWS account.
  suffix = "-${var.environment}"

  # Give prod's Secrets Manager secrets a deletion recovery window; let dev
  # recycle immediately so teardown/redeploy isn't blocked for days.
  secret_recovery_days = var.environment == "prod" ? 7 : 0
}
