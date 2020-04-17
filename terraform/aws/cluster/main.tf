provider "aws" {
  profile    = "default"
  region     = "$${region}"
  version = "~> 2.57"
}

terraform {
  backend "s3" {
    bucket = "konstellation"
    key    = "terraform/aws/$${region}/$${cluster}/cluster.tfstate"
  }
}
