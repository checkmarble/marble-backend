
import {
  id = local.project_id
  to = google_project.default
}

import {
  id = "projects/${local.project_id}"
  to = google_firebase_project.default
}

import {
  id = local.environment.marble_cloud_sql.name
  to = google_sql_database_instance.marble
}

import {
  id = "projects/${local.project_id}/instances/${local.environment.marble_cloud_sql.name}/databases/marble"
  to = google_sql_database.marble
}

import {
  id = "projects/${local.project_id}/serviceAccounts/marble-backend-cloud-run@${local.project_id}.iam.gserviceaccount.com"
  to = google_service_account.backend_service_account
}

import {
  id = "projects/${local.project_id}/serviceAccounts/marble-frontend-cloud-run@${local.project_id}.iam.gserviceaccount.com"
  to = google_service_account.frontend_service_account
}

import {
  id = "projects/${local.project_id}/secrets/AWS_SECRET_KEY"
  to = google_secret_manager_secret.aws_secret_key
}

import {
  id = "projects/${local.project_id}/secrets/AWS_ACCESS_KEY"
  to = google_secret_manager_secret.aws_access_key
}

import {
  id = "projects/${local.project_id}/secrets/AUTHENTICATION_JWT_SIGNING_KEY"
  to = google_secret_manager_secret.authentication_jwt_signing_key
}


import {
  id = "projects/${local.project_id}/secrets/POSTGRES_PASSWORD"
  to = google_secret_manager_secret.postgres_password
}

# no reason to import, just create a new one
# import {
#     id = "data-ingestion-${local.project_id}"
#     to = google_storage_bucket.data_ingestion
# }

import {
  id = "projects/${local.project_id}/locations/${local.location}/services/marble-backend"
  to = google_cloud_run_v2_service.backend
}


import {
  id = "projects/${local.project_id}/serviceAccounts/github-action-deploy@${local.project_id}.iam.gserviceaccount.com"
  to = google_service_account.github_action_service_account
}

import {
  id = "projects/${local.project_id}/config"
  to = google_identity_platform_config.auth
}

# import {
#   id = "projects/${local.project_id}/defaultSupportedIdpConfigs/google.com"
#   to = google_identity_platform_default_supported_idp_config.google
# }

import {
  id = "projects/${local.project_id}/webApps/${local.environment.firebase.backend_app_id}"
  to = google_firebase_web_app.backoffice
}

import {
  id = "projects/${local.project_id}/webApps/${local.environment.firebase.frontend_app_id}"
  to = google_firebase_web_app.frontend
}

import {
  id = "projects/${local.project_id}/sites/${local.environment.backoffice.firebase_site_id}"
  to = google_firebase_hosting_site.backoffice
}


# import {
#   id = "projects/${local.project_id}/sites/${local.environment.frontend.firebase_site_id}/customDomains/${local.environment.frontend.domain}"
#   to = google_firebase_hosting_custom_domain.frontend
# }
