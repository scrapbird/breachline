terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Account-wide layer: applied once per AWS account, NOT per environment, so
  # the state key is fixed rather than supplied per-env by the Makefile.
  backend "s3" {
    bucket       = "breachline-state-uf308ht4"
    key          = "account/terraform.tfstate"
    region       = "us-east-2"
    encrypt      = true
    use_lockfile = true
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      project   = "breachline"
      component = "account"
    }
  }
}
