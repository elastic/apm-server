terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">=4.27.0"
    }
    ec = {
      source  = "elastic/ec"
      version = "0.5.1"
    }
  }
}

provider "ec" {}

module "ec_deployment" {
  source = "../infra/terraform/modules/ec_deployment"

  region        = var.ess_region
  stack_version = var.stack_version

  deployment_template    = var.deployment_template
  deployment_name_prefix = "rally-cloud-testing"

  apm_server_size       = 0
  apm_server_zone_count = 0

  elasticsearch_size       = var.elasticsearch_size
  elasticsearch_zone_count = var.elasticsearch_zone_count

  custom_apm_integration_pkg_path = var.custom_apm_integration_pkg_path
}

module "rally_workers" {
  source = "../infra/terraform/modules/rally_workers"

  gcp_project  = var.gcp_project
  gcp_region   = var.gcp_region
  machine_type = var.rally_machine_type

  resource_prefix      = var.rally_workers_resource_prefix
  rally_dir            = var.rally_dir
  rally_worker_count   = var.rally_worker_count
  rally_cluster_status = var.rally_cluster_status
  rally_bulk_size      = var.rally_bulk_size
  rally_bulk_clients   = var.rally_bulk_clients

  elasticsearch_url      = module.ec_deployment.elasticsearch_url
  elasticsearch_username = module.ec_deployment.elasticsearch_username
  elasticsearch_password = module.ec_deployment.elasticsearch_password
}
