## General configuration

variable "user_name" {
  description = "Required username to use for prefixes"
  type        = string
}

## Deployment configuration

variable "ess_region" {
  default     = "gcp-us-west2"
  description = "Optional ESS region where the deployment will be created. Defaults to gcp-us-west2"
  type        = string
}

variable "deployment_template" {
  default     = "gcp-compute-optimized-v2"
  description = "Optional deployment template. Defaults to the CPU optimized template for GCP"
  type        = string
}

variable "stack_version" {
  default     = "latest"
  description = "Optional stack version"
  type        = string
}

variable "apm_server_size" {
  default     = "1g"
  type        = string
  description = "Optional apm server instance size"
}

variable "apm_server_zone_count" {
  default     = 1
  type        = number
  description = "Optional apm server zone count"
}

variable "elasticsearch_size" {
  default     = "8g"
  type        = string
  description = "Optional Elasticsearch instance size"
}

variable "elasticsearch_zone_count" {
  default     = 2
  type        = number
  description = "Optional Elasticsearch zone count"
}

variable "docker_image_tag_override" {
  default = {
    "elasticsearch" : "",
    "kibana" : "",
    "apm" : "",
  }
  description = "Optional docker image tag override"
  type        = map(string)
}

variable "docker_image_override" {
  default = {
    "elasticsearch" : "docker.elastic.co/cloud-release/elasticsearch-cloud-ess",
    "kibana" : "docker.elastic.co/cloud-release/kibana-cloud",
    "apm" : "docker.elastic.co/cloud-release/elastic-agent-cloud",
  }
  type = map(string)
}

## Worker configuraiton

variable "worker_region" {
  default     = "us-west-2"
  description = "Optional ESS region where the deployment will be created. Defaults to us-west-2 (AWS)"
  type        = string
}

variable "apmbench_bin_path" {
  default     = "../../systemtest/cmd/apmbench"
  type        = string
  description = "Optional path to the apmbench binary"
}

variable "worker_instance_type" {
  default     = "c6i.large"
  type        = string
  description = "Optional instance type to use for the worker VM"
}

variable "private_key" {
  default = "./id_rsa_terraform"
  type    = string
}

variable "public_key" {
  default = "./id_rsa_terraform.pub"
  type    = string
}

# CI variables
variable "BRANCH" {
  description = "Branch name or pull request for tagging purposes"
  default = "unknown-branch"
}

variable "BUILD_ID" {
  description = "Build ID in the CI for tagging purposes"
  default = "unknown-build"
}

variable "CREATED_DATE" {
  description = "Creation date in epoch time for tagging purposes"
  default = "unknown-date"
}

variable "ENVIRONMENT" {
  default = "unknown-environment"
}

variable "REPO" {
  default = "unknown-repo-name"
}
