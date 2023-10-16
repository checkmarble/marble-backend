locals {

  environments = {
    default = {
      project_id                    = "marble-test-terraform"
      terraform_service_account_key = "../service-account-key/marble-test-terraform-ecc0d390a523.json"
    }

    staging = {
      project_id = "tokyo-country-381508"
      terraform_service_account_key = "../service-account-key/tokyo-country-381508-1aa0f843ec5b.json"
    }
  }
}
