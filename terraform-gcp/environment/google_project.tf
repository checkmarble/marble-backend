
resource "google_project" "default" {
  project_id = local.project_id
  org_id     = 577386927588

  name = data.google_project.project.name

  labels = {
    "firebase" = "enabled"
  }
}

# import {
#   id = var.gcp_project
#   to = google_project.default
# }

data "google_project" "project" {
  project_id = local.project_id
}

# Enables Firebase services for the new project created above.
resource "google_firebase_project" "default" {
  project = local.project_id

  # Waits for the required APIs to be enabled.
  depends_on = [
    google_project_service.services
  ]
}


resource "google_project_service" "services" {
  for_each = toset([
    "cloudresourcemanager.googleapis.com",
    "storage.googleapis.com",        # to create cloud storage buckets
    "secretmanager.googleapis.com",  # to create secrets
    "sqladmin.googleapis.com",       # to create cloud sql instances
    "run.googleapis.com",            # to create cloud run services
    "iam.googleapis.com",            # to create service accounts
    "cloudscheduler.googleapis.com", # to create cloud scheduler jobs
    "firebase.googleapis.com",       # to create firebase projects
    # "firebasehosting.googleapis.com", # to create firebase hosting sites (unused)
    "identitytoolkit.googleapis.com", # to setup firebase authentication using google as an identity provider
  ])
  project = local.project_id
  service = each.key

  disable_on_destroy = false
}