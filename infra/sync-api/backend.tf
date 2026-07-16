terraform {
  # State key is supplied per-environment at init time via -backend-config,
  # e.g. -backend-config="key=sync-api/dev/terraform.tfstate". See the root
  # Makefile's tf-sync-api target.
  backend "s3" {
    bucket       = "breachline-state-uf308ht4"
    region       = "us-east-2"
    use_lockfile = true
  }
}
