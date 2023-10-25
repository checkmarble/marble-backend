resource "google_cloud_run_v2_service" "backend" {
  name     = "marble-backend"
  location = local.location
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    scaling {
      min_instance_count = 1
      max_instance_count = 100
    }

    max_instance_request_concurrency = 80
    service_account                  = google_service_account.backend_service_account.email
    # timeout = 

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.marble.connection_name]
      }
    }

    containers {
      image = local.environment.backend.image

      # Uncomment and deploy to add an admin
      # env {
      #   name  = "MARBLE_ADMIN_EMAIL"
      #   value = "pascal@checkmarble.com"
      # }

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
        value = local.environment.env_display_name
      }

      env {
        name  = "AWS_REGION"
        value = "eu-west-3"
      }

      env {
        name = "AWS_SECRET_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.aws_secret_key.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "AWS_ACCESS_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.aws_access_key.secret_id
            version = "latest"
          }
        }
      }

      env {
        name  = "GCS_INGESTION_BUCKET"
        value = google_storage_bucket.data_ingestion.name
      }

      env {
        name = "AUTHENTICATION_JWT_SIGNING_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.authentication_jwt_signing_key.secret_id
            version = "latest"
          }
        }
      }

      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }

      args = ["--server"]

      ports {
        name           = "http1"
        container_port = 80
      }

      startup_probe {
        tcp_socket {
          port = 80
        }
      }

      liveness_probe {
        http_get {
          path = "/liveness"
        }
      }
    }
  }
  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_secret_manager_secret_iam_member.secret_access_postgres_password,
    google_secret_manager_secret_iam_member.secret_access_aws_secret_key,
    google_secret_manager_secret_iam_member.secret_access_aws_access_key,
    google_secret_manager_secret_iam_member.secret_access_authentication_jwt_signing_key,
  ]

  # lifecycle {
  #   ignore_changes = [template[0].containers[0].image]
  # }
}

# Allow unauthenticated invocations of cloud run service
resource "google_cloud_run_service_iam_binding" "backend_allow_unauthenticated_invocations" {
  location = google_cloud_run_v2_service.backend.location
  service  = google_cloud_run_v2_service.backend.name
  role     = "roles/run.invoker"
  members = [
    "allUsers"
  ]
}
