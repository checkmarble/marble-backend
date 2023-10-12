
resource "google_project" "infra" {
  project_id = local.project_id
  org_id     = 577386927588

  name = data.google_project.infra.name
}

data "google_project" "infra" {
  project_id = local.project_id
}

resource "google_project_service" "storage_googleapis" {
  service            = "storage.googleapis.com" # to create cloud storage buckets
  disable_on_destroy = false
}

resource "google_project_service" "artifactregistry_googleapis" {
  service            = "artifactregistry.googleapis.com" # to create artifact registry repository
  disable_on_destroy = false
}
