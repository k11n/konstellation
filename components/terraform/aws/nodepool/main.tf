provider "aws" {
  profile = "default"
  region  = "$${region}"
  version = "~> 2.57"
}

terraform {
  backend "s3" {
    bucket = "$${state_bucket}"
    region = "$${state_bucket_region}"
    key    = "terraform/aws/$${region}/$${cluster}/nodepools/$${nodepool}.tfstate"
  }
}
