terraform {
  backend "s3" {
    bucket               = "sean-terraform"
    region               = "us-east-1"
    workspace_key_prefix = "aws/prosoche"
    key                  = "terraform.tfstate"
  }
}

provider "aws" {
  region = "us-east-1"
  default_tags {
    tags = {
      project   = "prosoche"
      workspace = terraform.workspace
    }
  }
}
