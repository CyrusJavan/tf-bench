resource "random_id" "id" {
  count       = 10
  byte_length = 16
}
provider "aws" {
  version = "~> 3.0"
  region  = "us-east-1"
}
resource "aws_s3_bucket" "b" {
  bucket = "my-tf-test-bucket-${random_id.id[0].dec}"
  acl    = "private"

  tags = {
    Name        = "My bucket"
    Environment = "Dev"
  }
}
terraform {
  required_providers {
    aviatrix = {
      source  = "aviatrixsystems/aviatrix"
    }
  }
}
provider "aviatrix" { skip_version_validation = true }
resource "aviatrix_vpc" "aws_vpc" {
  cloud_type           = 1
  account_name         = "aws-primary-acc"
  region               = "us-west-1"
  name                 = "aws-vpc"
  cidr                 = "10.0.0.0/16"
  aviatrix_transit_vpc = false
  aviatrix_firenet_vpc = false
}
