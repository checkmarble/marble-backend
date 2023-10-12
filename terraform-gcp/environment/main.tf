terraform {
  required_providers {
    google = {
      source  = "hashicorp/google-beta"
      version = "~> 4.51"
    }
  }
}

provider "google" {
  credentials = file(var.terraform_service_account_key)
  project     = local.project_id
  region      = local.location
}
