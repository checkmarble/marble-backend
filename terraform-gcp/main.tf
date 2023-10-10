terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "4.51.0"
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

data "google_project" "project" {
}

resource "google_project_service" "services" {
  for_each = toset([
    "cloudresourcemanager.googleapis.com",
    "secretmanager.googleapis.com",  // to create secrets
    "sqladmin.googleapis.com",       // to create cloud sql instances
    "run.googleapis.com",            // to create cloud run services
    "iam.googleapis.com",            // to create service accounts
    "cloudscheduler.googleapis.com", // to create cloud scheduler jobs
  ])
  project = data.google_project.project.project_id
  service = each.value
}
