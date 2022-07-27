terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">=4.27.0"
    }
  }
  backend "gcs" {
    bucket = "apmsoak-tf"
    prefix = "/tfstate"
  }
}

module "soaktest_workers" {
  source = "../infra/terraform/modules/soaktest_workers"

  gcp_project = var.gcp_project
  gcp_region  = var.gcp_region
  gcp_zone    = var.gcp_zone

  apmsoak_bin_path = var.apmsoak_bin_path

  apm_server_url              = var.apm_server_url
  apm_secret_token            = var.apm_secret_token
  apm_loadgen_max_rate        = var.apm_loadgen_max_rate
  apm_loadgen_agents_replicas = var.apm_loadgen_agents_replicas

  elastic_agent_version  = var.elastic_agent_version
  fleet_url              = var.fleet_url
  fleet_enrollment_token = var.fleet_enrollment_token
}
