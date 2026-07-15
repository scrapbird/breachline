terraform {
  # State key is supplied per-environment at init time via -backend-config,
  # e.g. -backend-config="key=sync-api/dev/terraform.tfstate". See the root
  # Makefile's tf-sync-api target.
  backend "s3" {
    bucket       = "scrappy-tfstate"
    region       = "ap-southeast-2"
    use_lockfile = true
  }
}
