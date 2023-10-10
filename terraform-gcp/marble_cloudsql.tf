resource "google_sql_database_instance" "marble" {
  name             = "marble"
  region           = var.gcp_location
  database_version = "POSTGRES_14"
  settings {
    tier = "db-f1-micro"
  }

  deletion_protection = "true"
}

resource "google_sql_database" "marble" {
  name     = "marble"
  instance = google_sql_database_instance.marble.name
}

resource "google_sql_user" "users" {
  name     = "postgres"
  instance = google_sql_database_instance.marble.name
  password = google_secret_manager_secret_version.postgres_password_data.secret_data
}
