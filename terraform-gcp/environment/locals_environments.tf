locals {

  environments = {
    default = {
      project_id                    = "marble-test-terraform"
      terraform_service_account_key = "../service-account-key/marble-test-terraform-ecc0d390a523.json"

      marble_cloud_sql = {
        name     = "marble"
        location = local.location
        tier     = "db-f1-micro"
      }

      firebase = {
        backend_app_id  = "1:1055186671888:web:04ccd4d77997ddf1b5ad95"
        frontend_app_id = "1:1055186671888:web:cea23ba6e9d095edb5ad95"
      }

      env_display_name = "staging"

      backoffice_domain = ""
      frontend_domain   = ""
    }

    staging = {
      project_id                    = "tokyo-country-381508"
      terraform_service_account_key = "../service-account-key/tokyo-country-381508-1aa0f843ec5b.json"

      marble_cloud_sql = {
        name     = "marble-sandbox"
        location = "europe-west9"
        tier     = "db-custom-1-4096"
      }

      firebase = {
        backend_app_id  = "1:1047691849054:web:59e5df4b6dbdacbe60b3cf"
        frontend_app_id = "1:1047691849054:web:a5b69dd2ac584c1160b3cf"
      }

      env_display_name = "staging"

      backoffice_domain = "marble-backoffice-staging.web.app"
      frontend_domain   = "app.staging.checkmarble.com"
    }
  }
}
