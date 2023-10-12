
variable "project_id" {
  description = "name of the GCP project"
  type        = string
  default     = "marble-test-terraform"
}

variable "region" {
  description = "region of the GCP project"
  type        = string
  default     = "europe-west1"
}

variable "terraform_service_account_key" {
  description = "path to the service account key"
  type        = string
  default     = "../service-account-key/marble-test-terraform-ecc0d390a523.json"
}
