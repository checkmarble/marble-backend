resource "google_sql_database_instance" "marble" {
  name             = local.environment.marble_cloud_sql.name
  region           = local.environment.marble_cloud_sql.location
  database_version = "POSTGRES_14"
  settings {
    tier = local.environment.marble_cloud_sql.tier
    maintenance_window {
      day  = 1
      hour = 0
    }

    deletion_protection_enabled = true

    insights_config {
      query_insights_enabled = true
      # query_plans_per_minute  = 5
      # query_string_length     = 1024
      # record_application_tags = false
      # record_client_address   = false
    }
  }

  deletion_protection = true
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
