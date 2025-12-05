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

module "tags" {
  source  = "../../infra/terraform/modules/tags"
  project = "apm-server"
}

locals {
  ci_tags = {
    environment  = coalesce(var.ENVIRONMENT, "dev")
    repo         = coalesce(var.REPO, "apm-server")
    branch       = var.BRANCH
    build        = var.BUILD_ID
    created_date = var.CREATED_DATE
    subproject   = "smoke-test-supported-os"
  }
}

data "aws_vpc" "default" {
  default = true
}

module "ec_deployment" {
  source = "../../infra/terraform/modules/ec_deployment"
  region = var.region

  deployment_template    = "gcp-vector-search-optimized"
  deployment_name_prefix = "supported-os-standalone"

  apm_server_size = "1g"

  elasticsearch_size       = "1g"
  elasticsearch_zone_count = 1

  stack_version = var.stack_version
  tags          = merge(local.ci_tags, module.tags.tags)
}

module "standalone_apm_server" {
  source = "../../infra/terraform/modules/standalone_apm_server"

  vpc_id                   = data.aws_vpc.default.id
  aws_os                   = var.aws_os
  aws_provisioner_key_name = var.aws_provisioner_key_name

  elasticsearch_url      = module.ec_deployment.elasticsearch_url
  elasticsearch_username = module.ec_deployment.elasticsearch_username
  elasticsearch_password = module.ec_deployment.elasticsearch_password
  stack_version          = var.stack_version

  tags       = merge(local.ci_tags, module.tags.tags)
  ea_managed = false
}

variable "aws_os" {
  default     = ""
  description = "Optional aws ec2 instance OS"
  type        = string
}

variable "aws_provisioner_key_name" {
  default     = ""
  description = "Optional ssh key name to create the aws key pair and remote provision the ec2 instance"
  type        = string
}

variable "stack_version" {
  default     = "latest"
  description = "Optional stack version"
  type        = string
}

variable "region" {
  default     = "gcp-us-west2"
  description = "Optional ESS region where to run the smoke tests"
  type        = string
}

# CI variables
variable "BRANCH" {
  description = "Branch name or pull request for tagging purposes"
  default     = "unknown"
}

variable "BUILD_ID" {
  description = "Build ID in the CI for tagging purposes"
  default     = "unknown"
}

variable "CREATED_DATE" {
  description = "Creation date in epoch time for tagging purposes"
  default     = "unknown"
}

variable "ENVIRONMENT" {
  default = "unknown"
}

variable "REPO" {
  default = "unknown"
}

output "apm_secret_token" {
  value       = module.standalone_apm_server.apm_secret_token
  description = "The APM Server secret token"
  sensitive   = true
}

output "apm_server_url" {
  value       = module.standalone_apm_server.apm_server_url
  description = "The APM Server URL"
}

output "kibana_url" {
  value       = module.ec_deployment.kibana_url
  description = "The Kibana URL"
}

output "elasticsearch_url" {
  value       = module.ec_deployment.elasticsearch_url
  description = "The Elasticsearch URL"
}

output "elasticsearch_username" {
  value       = module.ec_deployment.elasticsearch_username
  sensitive   = true
  description = "The Elasticsearch username"
}

output "elasticsearch_password" {
  value       = module.ec_deployment.elasticsearch_password
  sensitive   = true
  description = "The Elasticsearch password"
}

output "stack_version" {
  value       = module.ec_deployment.stack_version
  description = "The matching stack pack version from the provided stack_version"
}

output "apm_server_ip" {
  value       = module.standalone_apm_server.apm_server_ip
  description = "The APM Server EC2 IP address"
}

output "apm_server_ssh_user" {
  value       = module.standalone_apm_server.apm_server_ssh_user
  description = "The SSH user for the APM Server EC2 instance"
}

output "apm_server_os" {
  value       = module.standalone_apm_server.apm_server_os
  description = "The operating system name for the APM Server EC2 instance"
}
