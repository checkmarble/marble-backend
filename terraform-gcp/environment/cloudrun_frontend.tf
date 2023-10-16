resource "google_cloud_run_v2_service" "frontend" {
  name     = "marble-frontend"
  location = local.location
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    scaling {
      min_instance_count = 0
      max_instance_count = 1
    }

    max_instance_request_concurrency = 80
    service_account                  = google_service_account.frontend_service_account.email

    containers {
      image = "europe-docker.pkg.dev/tokyo-country-381508/marble/marble-frontend:latest"

      env {
        name  = "ENV"
        value = "staging"
      }

      env {
        name  = "FIREBASE_API_KEY"
        value = data.google_firebase_web_app_config.frontend.api_key
      }

      env {
        name  = "FIREBASE_APP_ID"
        value = data.google_firebase_web_app_config.frontend.web_app_id
      }

      env {
        name  = "FIREBASE_AUTH_DOMAIN"
        value = data.google_firebase_web_app_config.frontend.auth_domain
      }

      env {
        name  = "FIREBASE_MESSAGING_SENDER_ID"
        value = data.google_firebase_web_app_config.frontend.messaging_sender_id
      }

      env {
        name  = "FIREBASE_PROJECT_ID"
        value = local.project_id
      }

      env {
        name  = "FIREBASE_STORAGE_BUCKET"
        value = data.google_firebase_web_app_config.frontend.storage_bucket
      }
      env {
        name  = "MARBLE_API_DOMAIN"
        value = google_cloud_run_v2_service.backend.uri
      }

      env {
        name  = "NODE_ENV"
        value = "production"
      }

      env {
        name  = "SESSION_MAX_AGE"
        value = "43200"
      }

      env {
        name  = "SESSION_SECRET"
        value = random_password.frontend_session_secret.result
      }

      ports {
        name           = "http1"
        container_port = 8080
      }

      startup_probe {
        tcp_socket {
          port = 8080
        }
      }

      # liveness_probe {
      #   http_get {
      #     # path = "/liveness"
      #   }
      # }
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }
}

resource "random_password" "frontend_session_secret" {
  length = 16
}

# Allow unauthenticated invocations of cloud run service
resource "google_cloud_run_service_iam_binding" "frontend_allow_unauthenticated_invocations" {
  location = google_cloud_run_v2_service.frontend.location
  service  = google_cloud_run_v2_service.frontend.name
  role     = "roles/run.invoker"
  members = [
    "allUsers"
  ]
}