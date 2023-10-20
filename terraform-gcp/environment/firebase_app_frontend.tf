resource "google_firebase_web_app" "frontend" {
  display_name = "Marble"

  deletion_policy = "DELETE"
}

data "google_firebase_web_app_config" "frontend" {
  web_app_id = google_firebase_web_app.frontend.app_id
}

# resource "google_firebase_hosting_custom_domain" "frontend" {
#   site_id = local.environment.frontend.firebase_site_id
#   custom_domain = local.environment.frontend.domain
#   # cert_preference = "GROUPED"
#   wait_dns_verification = false
# }
