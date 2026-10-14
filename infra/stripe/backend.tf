terraform {
  required_version = ">= 1.11" # payment link line_items.quantity is write-only

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    # Stripe's official provider. Pre-1.0, so pinned exactly: review the
    # changelog before bumping.
    stripe = {
      source  = "stripe/stripe"
      version = "0.3.0"
    }
  }

  # State key is supplied per-environment at init time via -backend-config,
  # e.g. -backend-config="key=stripe/dev/terraform.tfstate". See the root
  # Makefile's tf-stripe target.
  backend "s3" {
    bucket       = "breachline-state-uf308ht4"
    region       = "us-east-2"
    encrypt      = true
    use_lockfile = true
  }
}

# dev uses a Stripe sandbox key (sk_test_), prod the live key (sk_live_); both
# come from the vault via TF_VAR_stripe_api_key.
provider "stripe" {
  api_key = var.stripe_api_key
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      project     = "breachline"
      component   = "stripe"
      environment = var.environment
    }
  }
}
