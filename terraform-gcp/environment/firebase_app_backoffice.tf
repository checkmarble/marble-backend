resource "google_firebase_web_app" "backoffice" {
  display_name = "Marble BackOffice"

  deletion_policy = "DELETE"
}

data "google_firebase_web_app_config" "backoffice" {
  web_app_id = google_firebase_web_app.backoffice.app_id
}

resource "google_firebase_hosting_site" "backoffice" {
  site_id = local.environment.backoffice.firebase_site_id
}

resource "google_firebase_hosting_custom_domain" "backoffice" {
  site_id       = local.environment.backoffice.firebase_site_id
  custom_domain = local.environment.backoffice.domain
  # cert_preference = "GROUPED"
  # redirect_target = "app.domain.com"
  wait_dns_verification = false
}

