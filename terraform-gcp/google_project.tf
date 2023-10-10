
resource "google_project" "default" {
  project_id = data.google_project.project.project_id
  org_id     = 577386927588

  name = data.google_project.project.name

  labels = {
    "firebase" = "enabled"
  }
}

import {
  id = var.gcp_project
  to = google_project.default
}


data "google_project" "project" {
}


# Enables Firebase services for the new project created above.
resource "google_firebase_project" "default" {
  project = google_project.default.project_id

  # Waits for the required APIs to be enabled.
  depends_on = [
    google_project_service.services
  ]
}
