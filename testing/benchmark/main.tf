terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    ec = {
      source  = "elastic/ec"
      version = ">=0.4.0"
    }
    aws = {
      source  = "hashicorp/aws"
      version = ">=4.10.0"
    }
  }
}

provider "ec" {}

provider "aws" {
  region = var.worker_region
  default_tags {
    tags = {
      environment  = "ci"
      repo         = "apm-server"
      branch       = "benchmarks_2_job"
      build        = "1"      
    }
  }
}

locals {
  name_prefix = "${var.user_name}-bench"
}

module "ec_deployment" {
  source = "../infra/terraform/modules/ec_deployment"

  region        = var.ess_region
  stack_version = var.stack_version

  deployment_template    = var.deployment_template
  deployment_name_prefix = local.name_prefix

  apm_server_size       = var.apm_server_size
  apm_server_zone_count = var.apm_server_zone_count

  elasticsearch_size       = var.elasticsearch_size
  elasticsearch_zone_count = var.elasticsearch_zone_count

  docker_image              = var.docker_image_override
  docker_image_tag_override = var.docker_image_tag_override
}

module "benchmark_worker" {
  source = "../infra/terraform/modules/benchmark_executor"
  region = var.worker_region

  user_name = var.user_name

  apm_server_url   = module.ec_deployment.apm_url
  apm_secret_token = module.ec_deployment.apm_secret_token

  apmbench_bin_path = var.apmbench_bin_path
  instance_type     = var.worker_instance_type

  public_key        = var.public_key
  private_key       = var.private_key
}
