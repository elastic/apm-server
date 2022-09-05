terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">=4.27.0"
    }
    ec = {
      source  = "elastic/ec"
      version = ">=0.4.0"
    }
  }
}

provider "ec" {}

locals {
  docker_image_tag = regex("docker.elastic.co/.*:(.*)", file("${path.module}/../../docker-compose.yml"))[0]
  match            = regex("(?:(.*)(?:-.*)-(?:SNAPSHOT))|(.*)", local.docker_image_tag)
  stack_version    = local.match[0] != null ? format("%s-SNAPSHOT", local.match[0]) : local.match[1]
}

module "ec_deployment" {
  source = "../infra/terraform/modules/ec_deployment"

  region        = var.ess_region
  stack_version = local.stack_version

  deployment_template    = var.deployment_template
  deployment_name_prefix = "rally-cloud-testing"

  apm_server_size       = 0
  apm_server_zone_count = 0

  elasticsearch_size       = var.elasticsearch_size
  elasticsearch_zone_count = var.elasticsearch_zone_count

  docker_image = var.docker_image_override
  docker_image_tag_override = {
    "elasticsearch" : coalesce(var.docker_image_tag_override["elasticsearch"], local.docker_image_tag),
    "kibana" : coalesce(var.docker_image_tag_override["kibana"], local.docker_image_tag),
    "apm": "",
  }
}

module "rally_workers" {
  source = "../infra/terraform/modules/rally_workers"

  gcp_project = var.gcp_project
  gcp_region  = var.gcp_region

  resource_prefix = var.rally_workers_resource_prefix

  rally_dir            = var.rally_dir
  rally_worker_count   = var.rally_worker_count
  rally_cluster_status = var.rally_cluster_status
  rally_bulk_size      = var.rally_bulk_size

  elasticsearch_url      = module.ec_deployment.elasticsearch_url
  elasticsearch_username = module.ec_deployment.elasticsearch_username
  elasticsearch_password = module.ec_deployment.elasticsearch_password
}
