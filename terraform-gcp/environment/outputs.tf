output "frontend_uri" {
  value = google_cloud_run_v2_service.frontend.uri
}

output "backend_uri" {
  value = google_cloud_run_v2_service.backend.uri
}

output "backoffice_url" {
  value = google_firebase_hosting_site.backoffice.default_url
}
