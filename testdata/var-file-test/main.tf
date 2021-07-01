resource "random_id" "id" {
  count       = var.random_count
  byte_length = 16
}
provider "aws" {
  region = "us-east-1"
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
      source = "aviatrixsystems/aviatrix"
    }
    aws = {
      version = "~> 3.0"
    }
    google = {
      version = "3.74.0"
    }
    azurerm = {
      version = "2.65.0"
    }
  }
  backend "s3" {
    bucket = "my-tf-test-bucket-randombits"
    key    = "var-file-test/tfstate"
    region = "us-east-1"
  }
}
provider "aviatrix" {
  skip_version_validation = true
  #  username                = data.aws_ssm_parameter.foo.value
  username      = var.aviatrix_username
  password      = var.aviatrix_password
  controller_ip = var.aviatrix_ip
}
#resource "aws_ssm_parameter" "foo" {
#  name  = "foo"
#  type  = "String"
#  value = var.aviatrix_username
#}
#data "aws_ssm_parameter" "foo" {
#  name = "foo"
#}
resource "aviatrix_vpc" "aws_vpc" {
  cloud_type           = 1
  account_name         = "aws-primary-acc"
  region               = "us-west-1"
  name                 = "aws-vpc"
  cidr                 = "10.0.0.0/16"
  aviatrix_transit_vpc = false
  aviatrix_firenet_vpc = false
}
provider "google" {
  project = var.google_project_id
  region  = var.google_region
}
resource "google_service_account" "default" {
  account_id   = "my-service-acc"
  display_name = "Service Account"
}
provider "azurerm" {
  features {}
}
resource "azurerm_resource_group" "example" {
  name     = "example-resources"
  location = "West Europe"
}
