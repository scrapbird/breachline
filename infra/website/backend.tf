terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # State key is supplied per-environment at init time via -backend-config,
  # e.g. -backend-config="key=website/dev/terraform.tfstate". See the root
  # Makefile's tf-website target.
  backend "s3" {
    bucket       = "breachline-state-uf308ht4"
    region       = "us-east-2"
    encrypt      = true
    use_lockfile = true
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      project     = "breachline"
      component   = "website"
      environment = var.environment
    }
  }
}

# Provider for ACM certificate (must be in us-east-1 for CloudFront)
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"

  default_tags {
    tags = {
      project     = "breachline"
      component   = "website"
      environment = var.environment
    }
  }
}
