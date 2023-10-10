
resource "google_storage_bucket" "data_ingestion" {
  name                     = "data-ingestion-${google_project.default.project_id}"
  location                 = var.gcp_location
  force_destroy            = true
  public_access_prevention = "enforced"

  uniform_bucket_level_access = true
}
