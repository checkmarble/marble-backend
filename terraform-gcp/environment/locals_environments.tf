locals {

  environments = {

    staging = {
      project_id                    = "tokyo-country-381508"
      terraform_service_account_key = "../service-account-key/tokyo-country-381508-1aa0f843ec5b.json"

      marble_cloud_sql = {
        name              = "marble-sandbox"
        location          = "europe-west9"
        tier              = "db-custom-1-4096"
        database_version  = "POSTGRES_14"
        availability_type = "ZONAL"
      }

      firebase = {
        backend_app_id  = "1:1047691849054:web:59e5df4b6dbdacbe60b3cf"
        frontend_app_id = "1:1047691849054:web:a5b69dd2ac584c1160b3cf"
      }

      env_display_name = "staging"

      backoffice = {
        firebase_site_id = "marble-backoffice-staging"
        domain           = "backoffice.staging.checkmarble.com"
      }

      frontend = {
        domain = "app.staging.checkmarble.com"
      }
    }


    production = {
      project_id                    = "marble-prod-1"
      terraform_service_account_key = "../service-account-key/marble-prod-1-c66803ae2892.json"

      marble_cloud_sql = {
        name              = "marble-prod"
        location          = local.location
        tier              = "db-custom-2-8192"
        database_version  = "POSTGRES_15"
        availability_type = "REGIONAL"
      }

      firebase = {
        backend_app_id  = "1:280431296971:web:ff089aa051073474f8f64e"
        frontend_app_id = "1:280431296971:web:bbdcd68f21ce8ee7f8f64e"
      }

      env_display_name = "production"

      backoffice = {
        firebase_site_id = "marble-backoffice-production"
        domain           = "marble-backoffice-production.web.app"
      }

      frontend = {
        domain = "app.checkmarble.com"
      }

    }

  }
}
