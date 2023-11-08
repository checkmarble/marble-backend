resource "google_cloud_run_v2_job" "migrations" {
  name     = "migrations"
  location = local.location

  template {

    template {
      timeout     = "3600s"
      max_retries = 0


      volumes {
        name = "cloudsql"
        cloud_sql_instance {
          instances = [google_sql_database_instance.marble.connection_name]
        }
      }

      # use backend service account
      service_account = google_service_account.backend_service_account.email

      containers {
        image = local.environment.backend.image

        env {
          name  = "PG_HOSTNAME"
          value = "/cloudsql/${local.project_id}:${google_sql_database_instance.marble.region}:${google_sql_database_instance.marble.name}"
        }

        env {
          name  = "PG_PORT"
          value = "5432"
        }

        env {
          name  = "PG_USER"
          value = google_sql_user.users.name
        }
        env {
          name = "PG_PASSWORD"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.postgres_password.secret_id
              version = "latest"
            }
          }
        }

        // we may get rid of this env variable
        env {
          name  = "ENV"
          value = "?staging??"
        }

        env {
          name  = "GCS_INGESTION_BUCKET"
          value = google_storage_bucket.data_ingestion.name
        }

        env {
          name  = "SEGMENT_WRITE_KEY"
          value = local.environment.segment_write_key
        }

        volume_mounts {
          name       = "cloudsql"
          mount_path = "/cloudsql"
        }

        args = ["--migrations"]
      }
    }
  }

  lifecycle {
    ignore_changes = [
      launch_stage,
    ]
  }
}
