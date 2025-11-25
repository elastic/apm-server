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
  project = "apm-server-benchmarks"
}

module "tags" {
  source = "../infra/terraform/modules/tags"
  # use the convention for team/shared owned resources if we are running in CI.
  # assume this is an individually owned resource otherwise.
  project = startswith(var.user_name, "benchci") ? local.project : "${local.project}-${var.user_name}"
}

provider "ec" {}

provider "aws" {
  region = var.worker_region
}

locals {
  name_prefix = "${coalesce(var.user_name, "unknown-user")}-bench"
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "3.14.0"

  name = "${var.user_name}-worker"
  cidr = var.vpc_cidr

  azs                = ["${var.worker_region}a"]
  public_subnets     = var.public_cidr
  enable_ipv6        = false
  enable_nat_gateway = false
  single_nat_gateway = false

  manage_default_security_group = true
  default_security_group_ingress = [
    {
      "from_port" : 0,
      "to_port" : 0,
      "protocol" : -1,
      "self" : true,
      "cidr_blocks" : "0.0.0.0/0",
    }
  ]
  default_security_group_egress = [
    {
      "from_port" : 0,
      "to_port" : 0,
      "protocol" : -1,
      "cidr_blocks" : "0.0.0.0/0",
    }
  ]

  tags = merge(local.ci_tags, module.tags.tags)
  vpc_tags = {
    Name = "vpc-${var.user_name}-worker"
  }
}

module "ec_deployment" {
  count  = var.run_standalone ? 0 : 1
  source = "../infra/terraform/modules/ec_deployment"

  region        = var.ess_region
  stack_version = var.stack_version

  deployment_template    = var.deployment_template
  deployment_name_prefix = local.name_prefix

  apm_server_size                        = var.apm_server_size
  apm_server_zone_count                  = var.apm_server_zone_count
  apm_index_shards                       = var.apm_shards
  drop_pipeline                          = var.drop_pipeline
  apm_server_expvar                      = true
  apm_server_pprof                       = true
  apm_server_tail_sampling               = var.apm_server_tail_sampling
  apm_server_tail_sampling_storage_limit = var.apm_server_tail_sampling_storage_limit
  apm_server_tail_sampling_sample_rate   = var.apm_server_tail_sampling_sample_rate

  elasticsearch_size              = var.elasticsearch_size
  elasticsearch_zone_count        = var.elasticsearch_zone_count
  elasticsearch_dedicated_masters = var.elasticsearch_dedicated_masters

  docker_image              = var.docker_image_override
  docker_image_tag_override = var.docker_image_tag_override

  tags = merge(local.ci_tags, module.tags.tags)
}

module "benchmark_worker" {
  source = "../infra/terraform/modules/benchmark_executor"

  vpc_id    = module.vpc.vpc_id
  region    = var.worker_region
  user_name = var.user_name

  apm_server_url   = var.run_standalone ? module.standalone_apm_server[0].apm_server_url : module.ec_deployment[0].apm_url
  apm_secret_token = var.run_standalone ? module.standalone_apm_server[0].apm_secret_token : module.ec_deployment[0].apm_secret_token

  apmbench_bin_path = var.apmbench_bin_path
  instance_type     = var.worker_instance_type

  public_key  = var.public_key
  private_key = var.private_key

  tags       = merge(local.ci_tags, module.tags.tags)
  depends_on = [module.standalone_apm_server, module.ec_deployment]
}

module "moxy" {
  count  = var.run_standalone ? 1 : 0
  source = "../infra/terraform/modules/moxy"

  vpc_id        = module.vpc.vpc_id
  instance_type = var.standalone_moxy_instance_size
  moxy_bin_path = var.moxy_bin_path

  aws_provisioner_key_name = var.private_key

  tags       = merge(local.ci_tags, module.tags.tags)
  depends_on = [module.vpc]
}


module "standalone_apm_server" {
  count  = var.run_standalone ? 1 : 0
  source = "../infra/terraform/modules/standalone_apm_server"

  vpc_id              = module.vpc.vpc_id
  aws_os              = "amzn2-ami-hvm-*-x86_64-ebs"
  apm_instance_type   = var.standalone_apm_server_instance_size
  apm_volume_type     = var.standalone_apm_server_volume_type
  apm_volume_size     = var.apm_server_tail_sampling ? coalesce(var.standalone_apm_server_volume_size, 60) : var.standalone_apm_server_volume_size
  apm_iops            = var.standalone_apm_server_iops
  apm_server_bin_path = var.apm_server_bin_path
  ea_managed          = false

  apm_server_tail_sampling               = var.apm_server_tail_sampling
  apm_server_tail_sampling_storage_limit = var.apm_server_tail_sampling_storage_limit
  apm_server_tail_sampling_sample_rate   = var.apm_server_tail_sampling_sample_rate

  aws_provisioner_key_name = var.private_key

  elasticsearch_url      = module.moxy[0].moxy_url
  elasticsearch_username = "elastic"
  elasticsearch_password = module.moxy[0].moxy_password

  tags       = merge(local.ci_tags, module.tags.tags)
  depends_on = [module.moxy]
}
