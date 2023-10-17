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

  // Can't use auto {}
  // gcp complain: "Due to an organization policy, automatic secret replication is not allowed."
  replication {
    user_managed {
      replicas {
        location = local.location
      }
    }
  }
}

resource "tls_private_key" "authentication_jwt_signing_key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "google_secret_manager_secret_version" "authentication_jwt_signing_key_data" {
  secret      = google_secret_manager_secret.authentication_jwt_signing_key.id
  secret_data = tls_private_key.authentication_jwt_signing_key.private_key_pem
}
