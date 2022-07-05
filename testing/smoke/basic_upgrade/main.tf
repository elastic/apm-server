terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    ec = {
      source  = "elastic/ec"
      version = ">=0.4.1"
    }
  }
}

provider "ec" {}

module "ec_deployment" {
  source = "../../infra/terraform/modules/ec_deployment"
  region = "gcp-us-west2"

  deployment_template    = "gcp-compute-optimized-v2"
  deployment_name_prefix = "smoke-upgrade"

  apm_server_size = "1g"

  elasticsearch_size       = "4g"
  elasticsearch_zone_count = 1

  stack_version       = var.stack_version
  integrations_server = var.integrations_server
}

variable "stack_version" {
  # By default match the latest available 7.17.x
  default     = "7.17.[0-9]?([0-9])$"
  description = "Optional stack version"
  type        = string
}

variable "integrations_server" {
  default     = false
  description = "Optionally use the integrations_server resource"
  type        = bool
}

output "apm_secret_token" {
  value       = module.ec_deployment.apm_secret_token
  description = "The APM Server secret token"
  sensitive   = true
}

output "apm_server_url" {
  value       = module.ec_deployment.apm_url
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
