resource "google_cloud_run_v2_job" "scheduled_executer" {
  name     = "scheduled-executer"
  location = local.location

  template {

    template {
      timeout     = "7200s"
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

        volume_mounts {
          name       = "cloudsql"
          mount_path = "/cloudsql"
        }

        args = ["--scheduler"]
      }
    }
  }

  lifecycle {
    ignore_changes = [
      launch_stage,
    ]
  }
}

// source of inspiration: https://github.com/chainguard-dev/terraform-google-cron/blob/main/main.tf
resource "google_cloud_scheduler_job" "scheduled_executer_cron" {
  name             = "scheduled_executer-cron"
  schedule         = "* * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "320s"


  http_target {
    http_method = "POST"
    uri         = "https://${local.location}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${local.project_id}/jobs/${google_cloud_run_v2_job.scheduled_executer.name}:run"

    oauth_token {
      service_account_email = google_service_account.backend_service_account.email
    }
  }
  depends_on = [google_cloud_run_v2_job.scheduled_executer, google_project_iam_member.backend_service_account_cron_run_invoker]
}
