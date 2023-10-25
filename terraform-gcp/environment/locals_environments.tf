locals {

  sentry_auth = {
    dsn = "https://aca3c26a7cb6d88c8317dfccba4726a0@o4506060675088384.ingest.sentry.io/4506060678037504"
  }

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
        backoffice_app_id = "1:1047691849054:web:59e5df4b6dbdacbe60b3cf"
        frontend_app_id   = "1:1047691849054:web:a5b69dd2ac584c1160b3cf"
      }

      env_display_name = "staging"

      backoffice = {
        firebase_site_id = "marble-backoffice-staging"
        domain           = "backoffice.staging.checkmarble.com"
      }

      frontend = {
        image = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-frontend:latest"
        # fix login popup that open another login popup
        # domain = "app.staging.checkmarble.com"
        domain                    = "tokyo-country-381508.firebaseapp.com"
        another_authorized_domain = "app.staging.checkmarble.com"
      }

      backend = {
        image = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-backend:latest"
        url   = "https://api.staging.checkmarble.com"
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
        backoffice_app_id = "1:280431296971:web:ff089aa051073474f8f64e"
        frontend_app_id   = "1:280431296971:web:bbdcd68f21ce8ee7f8f64e"
      }

      env_display_name = "production"

      backoffice = {
        firebase_site_id = "marble-backoffice-production"
        domain           = "marble-backoffice-production.web.app"
      }

      frontend = {
        image                     = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-frontend:v0.0.7"
        domain                    = "marble-prod-1.firebaseapp.com"
        another_authorized_domain = "app.checkmarble.com"
      }

      backend = {
        image = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-backend:v0.0.19"
        url   = "https://api.checkmarble.com"
      }
    }

  }
}
