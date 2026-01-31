terraform {
  backend "s3" {
    bucket = "scrappy-tfstate"
    key    = "sync-api/terraform.tfstate"
    region = "ap-southeast-2"
    use_lockfile = true
  }
}
