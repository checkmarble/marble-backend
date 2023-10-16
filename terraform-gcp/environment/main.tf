terraform {
  required_providers {
    google = {
      source  = "hashicorp/google-beta"
      version = "~> 4.51"
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
