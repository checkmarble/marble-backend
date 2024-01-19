locals {

  sentry_auth = {
    dsn = "https://aca3c26a7cb6d88c8317dfccba4726a0@o4506060675088384.ingest.sentry.io/4506060678037504"
  }

  environments = {
    staging = {
      project_id                    = "tokyo-country-381508"
      terraform_service_account_key = "../service-account-key/tokyo-country-381508.json"

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

      segment_write_key = {
        frontend = "ansHLeOAaKAU50Hu6euGrJ6mhcKomveC"
        backend  = "DD4qD8Usg1zfsjnLweSCYYrXQRLzYiGB"
      }

      backoffice = {
        firebase_site_id = "marble-backoffice-staging"
        domain           = "backoffice.staging.checkmarble.com"
      }

      frontend = {
        image  = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-frontend:latest"
        domain = "app.staging.checkmarble.com"
        url    = "https://app.staging.checkmarble.com"
      }

      backend = {
        image  = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-backend:latest"
        url    = "https://api.staging.checkmarble.com"
        domain = "api.staging.checkmarble.com"
      }
    }


    production = {
      project_id                    = "marble-prod-1"
      terraform_service_account_key = "../service-account-key/marble-prod-1.json"

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

      segment_write_key = {
        frontend = "bEDdodQ5CBrUFeaHvVClSf0BfuWYyzeN"
        backend  = "JeAT8VCKjBs7gVrFY23PG7aSMPqcvNFE"
      }

      backoffice = {
        firebase_site_id = "marble-backoffice-production"
        domain           = "marble-backoffice-production.web.app"
      }

      frontend = {
        image  = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-frontend:v0.0.18"
        domain = "app.checkmarble.com"
        url    = "https://app.checkmarble.com"
      }

      backend = {
        image  = "europe-west1-docker.pkg.dev/marble-infra/marble/marble-backend:v0.0.32"
        url    = "https://api.checkmarble.com"
        domain = "api.checkmarble.com"
      }
    }

  }
}
