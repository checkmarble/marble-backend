
resource "google_service_account" "github_action_service_account" {
  project      = local.project_id
  account_id   = "github-action-deploy"
  display_name = "Github action service account"
}

# Allow github action service account to invoke cloud run jobs
resource "google_project_iam_member" "github_action_cron_run_invoker" {
  project = local.project_id
  role    = "roles/run.invoker"
  member  = google_service_account.github_action_service_account.member
}

# resource "google_project_iam_member" "github_action_cron_run_invoker" {
#   project = local.project_id
#   role    = "roles/serviceusage.apiKeysViewer"
#   member  = google_service_account.github_action_service_account.member
# }


// removed: Artifact Registry Administrator
// removed: Cloud Functions Developer
// removed: Cloud Scheduler Admin
// removed: Firebase Authentication Admin
// removed: Firebase Hosting Admin
// removed: Service Account User
// removed: Cloud Run Admin
// removed: Storage Admin
