
resource "google_secret_manager_secret_iam_member" "secret_access_postgres_password" {
  secret_id  = google_secret_manager_secret.postgres_password.id
  role       = "roles/secretmanager.secretAccessor"
  member     = google_service_account.backend_service_account.member
  depends_on = [google_secret_manager_secret_version.postgres_password_data]
}

resource "google_secret_manager_secret_iam_member" "secret_access_aws_secret_key" {
  secret_id = google_secret_manager_secret.aws_secret_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.backend_service_account.member
}

resource "google_secret_manager_secret_iam_member" "secret_access_aws_access_key" {
  secret_id = google_secret_manager_secret.aws_access_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.backend_service_account.member
}

resource "google_secret_manager_secret_iam_member" "secret_access_authentication_jwt_signing_key" {
  secret_id  = google_secret_manager_secret.authentication_jwt_signing_key.id
  role       = "roles/secretmanager.secretAccessor"
  member     = google_service_account.backend_service_account.member
  depends_on = [google_secret_manager_secret_version.authentication_jwt_signing_key_data]
}
