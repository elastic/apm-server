terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    ec = {
      source  = "elastic/ec"
      version = ">=0.4.0"
    }
    aws = {
      source  = "hashicorp/aws"
      version = ">=4.17.1"
    }
  }
}

locals {
  ci_tags = {
    environment  = var.ENVIRONMENT
    repo         = var.REPO
    branch       = var.BRANCH
    build        = var.BUILD_ID
    created_date = var.CREATED_DATE
  }

  mandatory_tags = {
    division  = "engineering"
    org       = "obs"
    team      = "apm-server"
    project   = "benchmarks"
  }
}

provider "ec" {}

provider "aws" {
  region = var.worker_region
  default_tags {
    tags = local.ci_tags
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

  tags = merge(local.ci_tags, local.mandatory_tags)
}

module "benchmark_worker" {
  source = "../infra/terraform/modules/benchmark_executor"
  region = var.worker_region

  user_name = var.user_name

  apm_server_url   = module.ec_deployment.apm_url
  apm_secret_token = module.ec_deployment.apm_secret_token

  apmbench_bin_path = var.apmbench_bin_path
  instance_type     = var.worker_instance_type

  public_key  = var.public_key
  private_key = var.private_key
}
