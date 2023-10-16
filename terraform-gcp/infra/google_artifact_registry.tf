
resource "google_artifact_registry_repository" "marble" {

  repository_id = "marble"
  description   = "Build artifacts used by environments"
  format        = "DOCKER"

  depends_on = [google_project_service.artifactregistry_googleapis]
}

resource "google_artifact_registry_repository_iam_member" "cloud_run_service_agent_of_environments" {

  for_each   = local.environments
  location   = google_artifact_registry_repository.marble.location
  repository = google_artifact_registry_repository.marble.name
  role       = "roles/artifactregistry.reader"

  // cloud run service agent. doc: https://cloud.google.com/iam/docs/service-agents#google-cloud-run-service-agent
  member = "serviceAccount:service-${each.value}@serverless-robot-prod.iam.gserviceaccount.com"
}

resource "google_artifact_registry_repository_iam_member" "github_agent_of_environments" {

  for_each   = local.environments
  location   = google_artifact_registry_repository.marble.location
  repository = google_artifact_registry_repository.marble.name
  role       = "roles/artifactregistry.repoAdmin"

  // manually created github service account
  member = "serviceAccount:github-action-deploy@${each.key}.iam.gserviceaccount.com"
}
