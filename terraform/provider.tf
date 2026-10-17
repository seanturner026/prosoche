terraform {
  backend "s3" {
    bucket = "sean-terraform"
    region = "us-east-1"
    key    = "aws/prosoche/terraform.tfstate"
  }
}

provider "aws" {
  region = "us-east-1"
  default_tags {
    tags = {
      project = "prosoche"
    }
  }
}
