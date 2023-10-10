# firebase authentication using google as an identity provider

# The following official documentation proposes two methods:
# https://firebase.google.com/codelabs/firebase-terraform#5
# - manually set up Firebase Authentication in the console (https://console.firebase.google.com/)
# - set up Firebase Authentication via Terraform using Google Cloud Identity Platform (GCIP) APIs
# Unfortunately, both methods depends of manual actions, which is sad.
# So I used firebase console manually to enable google as an authentication provider

resource "google_identity_platform_config" "auth" {
  project = google_project.default.project_id
  #   autodelete_anonymous_users = true

  sign_in {
    allow_duplicate_emails = false

    anonymous {
      enabled = false
    }
  }

  authorized_domains = [
    # "localhost",
    # "marble-test-terraform.firebaseapp.com",
    # "marble-test-terraform.web.app",
    // TODO: small hack to extract domain, because binding of custom domain is not terraformed yet
    trimprefix(google_cloud_run_v2_service.frontend.uri, "https://"),
    trimprefix(google_firebase_hosting_site.backoffice.default_url, "https://"),
  ]
}

# resource "google_identity_platform_default_supported_idp_config" "google" {
#   enabled = true
#   idp_id  = "google.com"
#   client_id     = "client-id"
#   client_secret = "secret"
# }

# import {
#   id = "projects/${google_project.default.project_id}/config"
#   to = google_identity_platform_config.auth
# }
