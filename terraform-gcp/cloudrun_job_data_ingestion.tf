resource "google_cloud_run_v2_job" "data_ingestion" {
  name     = "data-ingestion"
  location = var.gcp_location

  template {

    template {
      timeout = "7200s"
      # max_retries = 0

      volumes {
        name = "cloudsql"
        cloud_sql_instance {
          instances = [google_sql_database_instance.marble.connection_name]
        }
      }

      # use backend service account
      service_account = google_service_account.backend_service_account.email

      containers {
        image = "europe-docker.pkg.dev/tokyo-country-381508/marble/marble-backend:latest"

        env {
          name  = "PG_HOSTNAME"
          value = "/cloudsql/${data.google_project.project.project_id}:${google_sql_database_instance.marble.region}:${google_sql_database_instance.marble.name}"
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

        args = ["--data-ingestion"]
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
resource "google_cloud_scheduler_job" "data_ingestion_cron" {
  name             = "data-ingestion-cron"
  schedule         = "* * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "320s"

  http_target {
    http_method = "POST"
    uri         = "https://${var.gcp_location}-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/${data.google_project.project.project_id}/jobs/${google_cloud_run_v2_job.data_ingestion.name}:run"

    oauth_token {
      service_account_email = google_service_account.backend_service_account.email
    }
  }
  depends_on = [google_cloud_run_v2_job.data_ingestion, google_project_iam_member.backend_service_account_cron_run_invoker]
}


#   - name: Deploy data ingestion job
#     run: |
#       gcloud beta run jobs deploy marble-backend-data-ingestion \
#         --quiet \
#         --image="europe-docker.pkg.dev/${{ vars.GCP_PROJECT_ID }}/marble/marble-backend:latest" \
#         --region="europe-west1" \
#         --args=-data-ingestion \
#         --service-account=marble-backend-cloud-run@${{ vars.GCP_PROJECT_ID }}.iam.gserviceaccount.com \
#         --set-env-vars=PG_HOSTNAME=/cloudsql/${{ vars.GCP_PROJECT_ID }}:${{ vars.DB_INSTANCE_REGION }}:${{ vars.DB_INSTANCE_NAME }},PG_USER=postgres,GOOGLE_CLOUD_PROJECT=${{ vars.GCP_PROJECT_ID }},ENV=${{ inputs.environment }},AWS_REGION=eu-west-3,GCS_INGESTION_BUCKET=${{ vars.GCS_INGESTION_BUCKET }} \
#         --set-secrets=PG_PASSWORD=POSTGRES_PASSWORD:latest,AWS_ACCESS_KEY=AWS_ACCESS_KEY:latest,AWS_SECRET_KEY=AWS_SECRET_KEY:latest \
#         --set-cloudsql-instances=${{ vars.GCP_PROJECT_ID }}:${{ vars.DB_INSTANCE_REGION }}:${{ vars.DB_INSTANCE_NAME }} \
#         --task-timeout=2h \
#         --max-retries=0

