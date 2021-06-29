terraform {
  backend "s3" {
    bucket = "my-tf-test-bucket-randombits"
    key    = "s3-backend-test/tfstate"
    region = "us-east-1"
  }
}
resource "random_id" "id" {
  count       = 10
  byte_length = 16
}
resource "random_pet" "pet" {
  count = 10
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
