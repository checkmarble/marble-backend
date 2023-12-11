resource "google_compute_global_address" "default" {
  name = "marble-${local.environment.env_display_name}-ip"
}

resource "google_compute_managed_ssl_certificate" "default" {
  name        = "marble-${local.environment.env_display_name}-certificate"
  description = "API and web app"
  type        = "MANAGED"

  managed {
    domains = [local.environment.frontend_domain, local.environment.backend_domain]
  }

  timeouts {}
}

resource "google_compute_region_network_endpoint_group" "backend_neg" {
  name                  = "backend-neg"
  network_endpoint_type = "SERVERLESS"
  region                = local.location
  cloud_run {
    service = google_cloud_run_v2_service.backend.name
  }
}


resource "google_compute_region_network_endpoint_group" "frontend_neg" {
  name                  = "frontend-neg"
  network_endpoint_type = "SERVERLESS"
  region                = local.location
  cloud_run {
    service = google_cloud_run_v2_service.frontend.name
  }
}

resource "google_compute_backend_service" "backend_service" {
  name = "${local.environment.env_display_name}-backend-service"

  protocol        = "HTTP"
  port_name       = "http"
  timeout_sec     = 30
  security_policy = google_compute_security_policy.policy.name

  backend {
    group = google_compute_region_network_endpoint_group.backend_neg.id
  }
}

resource "google_compute_backend_service" "frontend_service" {
  name = "${local.environment.env_display_name}-frontend-service"

  protocol        = "HTTP"
  port_name       = "http"
  timeout_sec     = 30
  security_policy = google_compute_security_policy.policy.name

  backend {
    group = google_compute_region_network_endpoint_group.frontend_neg.id
  }
}

resource "google_compute_url_map" "default" {
  name        = "marble-${local.environment.env_display_name}-load-balancer"
  description = "URL map for ${local.project_id}"

  default_service = google_compute_backend_service.frontend_service.id

  host_rule {
    hosts        = [local.environment.backend_domain]
    path_matcher = "backend"
  }
  path_matcher {
    name            = "backend"
    default_service = google_compute_backend_service.backend_service.id
  }
  host_rule {
    hosts        = [local.environment.frontend_domain]
    path_matcher = "frontend"
  }
  path_matcher {
    name            = "frontend"
    default_service = google_compute_backend_service.frontend_service.id
  }
}

resource "google_compute_target_https_proxy" "default" {
  name = "marble-${local.environment.env_display_name}-https-proxy"

  url_map = google_compute_url_map.default.id
  ssl_certificates = [
    "projects/marble-prod-1/global/sslCertificates/marble-load-balancer-v2",
    google_compute_managed_ssl_certificate.default.id,
  ]
}

resource "google_compute_global_forwarding_rule" "default" {
  name = "marble-${local.environment.env_display_name}-load-balancer"

  target     = google_compute_target_https_proxy.default.id
  port_range = "443"
  ip_address = google_compute_global_address.default.address
}

resource "google_compute_security_policy" "policy" {
  provider    = google-beta
  description = null
  name        = "${local.environment.env_display_name}-armor-policy"
  type        = "CLOUD_ARMOR"

  rule {
    action      = "allow"
    description = "allow calls made with host header"
    preview     = false
    priority    = 0
    match {
      versioned_expr = null
      expr {
        expression = "request.headers['Host'].endsWith(\".checkmarble.com\") || request.headers[':authority'].endsWith(\".checkmarble.com\")"
      }
    }
    preconfigured_waf_config {
    }
  }
  rule {
    action      = "deny(403)"
    description = "Default rule, higher priority overrides it"
    preview     = false
    priority    = 2147483647
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    preconfigured_waf_config {
    }
  }
  timeouts {
    create = null
    delete = null
    update = null
  }
}
