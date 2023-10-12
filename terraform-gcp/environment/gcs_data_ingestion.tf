
resource "google_storage_bucket" "data_ingestion" {
  name                     = "data-ingestion-${local.project_id}"
  location                 = local.location
  force_destroy            = true
  public_access_prevention = "enforced"

  uniform_bucket_level_access = true
}
