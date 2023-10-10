
variable "gcp_location" {
  description = "value of the location of the GCP project"
  type        = string
  default     = "europe-west1"
}

variable "gcp_project" {
  description = "name of the GCP project"
  type        = string
  default     = "marble-test-terraform"
}

variable "service_account_key" {
  description = "path to the service account key"
  type        = string
  default     = "service-account-key/marble-test-terraform-ecc0d390a523.json"
}
