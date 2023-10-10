terraform {
  required_providers {
    google = {
      source  = "hashicorp/google-beta"
      version = "~> 4.51"
    }
  }
}

provider "google" {
  credentials = file(var.service_account_key)

  project = var.gcp_project
  region  = var.gcp_location
}

# resource "google_compute_network" "vpc_network" {
#   name = "terraform-network"
# }

resource "google_project_service" "services" {
  for_each = toset([
    "cloudresourcemanager.googleapis.com",
    "secretmanager.googleapis.com",   # to create secrets
    "sqladmin.googleapis.com",        # to create cloud sql instances
    "run.googleapis.com",             # to create cloud run services
    "iam.googleapis.com",             # to create service accounts
    "cloudscheduler.googleapis.com",  # to create cloud scheduler jobs
    "firebase.googleapis.com",        # to create firebase projects
    "identitytoolkit.googleapis.com", # to setup firebase authentication using google as an identity provider
  ])
  project = google_project.default.project_id
  service = each.key

  disable_on_destroy = false
}
