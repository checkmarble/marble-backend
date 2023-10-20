terraform {
  required_providers {
    google = {
      source  = "hashicorp/google-beta"
      version = "~>5.2"
    }
  }
  backend "gcs" {
    bucket = "marble_terraform_tfstate"
    prefix = "environment"
  }
}

provider "google" {
  credentials = file(local.environment.terraform_service_account_key)
  project     = local.project_id
  region      = local.location
}
