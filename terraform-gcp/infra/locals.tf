locals {
  project_id = "marble-infra"
  location   = "europe-west1"

  # project id and project number of each environment
  environments = {
    "tokyo-country-381508"  = "1047691849054"
    "marble-prod-1"         = "280431296971"
  }
}
