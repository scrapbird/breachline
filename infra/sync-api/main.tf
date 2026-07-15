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

  # The SES domain identity (breachline.app) is one shared, account-level
  # resource. Only prod owns/verifies it; dev sends from the same verified
  # domain (noreply-dev@breachline.app) without re-declaring the identity, so
  # the two environments don't fight over ownership on apply/destroy.
  manage_ses_domain = var.environment == "prod"
}
