resource "google_firebase_web_app" "backoffice" {
  display_name = "backoffice"

  deletion_policy = "DELETE"
}

data "google_firebase_web_app_config" "backoffice" {
  web_app_id = google_firebase_web_app.backoffice.app_id
}

resource "google_firebase_hosting_site" "backoffice" {
  app_id  = local.environment.firebase.backend_app_id
  site_id = local.environment.backoffice.firebase_site_id
}
