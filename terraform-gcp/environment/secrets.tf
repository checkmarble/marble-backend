
resource "google_secret_manager_secret" "postgres_password" {
  secret_id = "POSTGRES_PASSWORD"
  replication {
    auto {}
  }
}

resource "random_password" "pg_password" {
  length = 16
}

resource "google_secret_manager_secret_version" "postgres_password_data" {
  secret      = google_secret_manager_secret.postgres_password.id
  secret_data = random_password.pg_password.result
}

resource "google_secret_manager_secret" "aws_secret_key" {
  secret_id = "AWS_SECRET_KEY"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "aws_access_key" {
  secret_id = "AWS_ACCESS_KEY"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "authentication_jwt_signing_key" {
  secret_id = "AUTHENTICATION_JWT_SIGNING_KEY"
  replication {
    auto {}
  }
}
