output "frontend_domain" {
  value = local.environment.frontend.domain
}
output "frontend_cloudrun_uri" {
  value = google_cloud_run_v2_service.frontend.uri
}

output "backoffice_domain" {
  value = local.environment.backoffice.domain
}

output "backoffice_firebase_uri" {
  value = google_firebase_hosting_site.backoffice.default_url
}

output "backend_uri" {
  value = google_cloud_run_v2_service.backend.uri
}
