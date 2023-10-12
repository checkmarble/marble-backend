
resource "google_storage_bucket" "terraform_tfstate" {
  name                     = "marble_terraform_tfstate"
  location                 = local.location
  force_destroy            = false
  public_access_prevention = "enforced"
  # uniform_bucket_level_access = true

  # encryption {
  #   default_kms_key_name = google_kms_crypto_key.terraform_state_bucket.id
  # }

  versioning {
    enabled = true
  }
  depends_on = [google_project_service.storage_googleapis]
}
