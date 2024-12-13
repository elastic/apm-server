terraform {
  required_version = ">= 0.12.29"

  required_providers {
    ec = {
      source  = "elastic/ec"
      version = "0.12.2"
    }
  }
}

variable "name" {
  type        = string
  description = "The deployment name"
}

variable "stack_version" {
  type        = string
  description = "The Elasticsearch version to bootstrap"
}

provider "ec" {
  endpoint = "https://public-api.qa.cld.elstc.co"
}

resource "ec_deployment" "example_minimal" {
  name = var.name

  region                 = "aws-eu-west-1"
  version                = var.stack_version
  deployment_template_id = "aws-storage-optimized"

  elasticsearch = {
    hot = {
      autoscaling = {}
    }

    ml = {
      autoscaling = {
        autoscale = true
      }
    }
  }

  kibana = {
    topology = {}
  }

  integrations_server = {}
}

output "deployment_id" {
  value = ec_deployment.example_minimal.id
}

output "apm_url" {
  value = ec_deployment.example_minimal.integrations_server.endpoints.apm
}

output "es_url" {
  value = ec_deployment.example_minimal.elasticsearch.https_endpoint
}

output "username" {
  value = ec_deployment.example_minimal.elasticsearch_username
}
output "password" {
  value     = ec_deployment.example_minimal.elasticsearch_password
  sensitive = true
}

output "kb_url" {
  value = ec_deployment.example_minimal.kibana.https_endpoint
}
