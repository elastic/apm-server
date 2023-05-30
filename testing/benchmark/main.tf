terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    ec = {
      source  = "elastic/ec"
      version = "0.5.1"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~>4.17"
    }
    time = {
      source  = "hashicorp/time"
      version = ">=0.9.1"
    }
  }
}

resource "time_static" "created_date" {}

locals {
  ci_tags = {
    environment  = var.ENVIRONMENT
    repo         = var.REPO
    branch       = var.BRANCH
    build        = var.BUILD_ID
    created_date = coalesce(var.CREATED_DATE, time_static.created_date.unix)
  }
}

module "tags" {
  source = "../infra/terraform/modules/tags"
  # use the convention for team/shared owned resources if we are running in CI.
  # assume this is an individually owned resource otherwise.
  project = startswith(var.user_name, "benchci") ? "benchmarks" : var.user_name
}

provider "ec" {}

provider "aws" {
  region = var.worker_region
}

locals {
  name_prefix = "${coalesce(var.user_name, "unknown-user")}-bench"
}

module "ec_deployment" {
  source = "../infra/terraform/modules/ec_deployment"

  region        = var.ess_region
  stack_version = var.stack_version

  deployment_template    = var.deployment_template
  deployment_name_prefix = local.name_prefix

  apm_server_size       = var.apm_server_size
  apm_server_zone_count = var.apm_server_zone_count
  apm_index_shards      = var.apm_shards
  drop_pipeline         = var.drop_pipeline
  apm_server_expvar     = true
  apm_server_pprof      = true

  elasticsearch_size              = var.elasticsearch_size
  elasticsearch_zone_count        = var.elasticsearch_zone_count
  elasticsearch_dedicated_masters = var.elasticsearch_dedicated_masters

  docker_image              = var.docker_image_override
  docker_image_tag_override = var.docker_image_tag_override

  tags = merge(local.ci_tags, module.tags.tags)
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

  tags = merge(local.ci_tags, module.tags.tags)
}
