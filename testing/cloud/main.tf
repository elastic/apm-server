terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    ec = {
      source  = "elastic/ec"
      version = "0.5.1"
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
  deployment_name_prefix = "apm-server-testing"

  apm_server_size       = var.apm_server_size
  apm_server_zone_count = var.apm_server_zone_count

  elasticsearch_size       = var.elasticsearch_size
  elasticsearch_zone_count = var.elasticsearch_zone_count

  observability_deployment = var.observability_deployment

  docker_image = var.docker_image_override
  docker_image_tag_override = {
    "elasticsearch" : coalesce(var.docker_image_tag_override["elasticsearch"], local.docker_image_tag),
    "kibana" : coalesce(var.docker_image_tag_override["kibana"], local.docker_image_tag),
    "apm" : coalesce(var.docker_image_tag_override["apm"], local.docker_image_tag)
  }
}
