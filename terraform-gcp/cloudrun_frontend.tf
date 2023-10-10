resource "google_cloud_run_v2_service" "frontend" {
  name     = "marble-frontend"
  location = var.gcp_location
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    scaling {
      min_instance_count = 1
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
        name = "FIREBASE_API_KEY"
        # TODO: to vars
        value = "AIzaSyAElc2shIKIrYzLSzWmWaZ1C7yEuoS-bBw"
      }

      env {
        name = "FIREBASE_APP_ID"
        # TODO: to vars
        value = "1:1047691849054:web:a5b69dd2ac584c1160b3cf"
      }

      env {
        name = "FIREBASE_AUTH_DOMAIN"
        # TODO: to vars
        value = "tokyo-country-381508.firebaseapp.com/"
      }

      env {
        name = "FIREBASE_MESSAGING_SENDER_ID"
        # TODO: to vars
        value = "1047691849054"
      }

      env {
        name = "FIREBASE_PROJECT_ID"
        # TODO: to vars
        value = "tokyo-country-381508"
      }
      env {
        name = "FIREBASE_STORAGE_BUCKET"
        # TODO: to vars
        value = "tokyo-country-381508.appspot.com"
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

  depends_on = [
    random_password.frontend_session_secret,
  ]
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
