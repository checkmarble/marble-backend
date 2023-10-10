resource "google_service_account" "frontend_service_account" {
  account_id   = "marble-frontend-cloud-run"
  display_name = "Marble FrontEnd Service Account"
}
