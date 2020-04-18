provider "aws" {
  profile    = "default"
  region     = "$${region}"
  version = "~> 2.57"
}

terraform {
  backend "s3" {
    bucket = "$${state_bucket}"
    key    = "terraform/aws/$${region}/vpc/$${vpc_cidr}.tfstate"
  }
}
