## General configuration

variable "user_name" {
  description = "Required username to use for prefixes"
  type        = string
}

variable "run_standalone" {
  default     = false
  description = "If set run benchmarks against standalone APM Server connected to moxy"
  type        = bool
}

## Deployment configuration

variable "ess_region" {
  default     = "gcp-us-west2"
  description = "Optional ESS region where the deployment will be created. Defaults to gcp-us-west2"
  type        = string
}

variable "deployment_template" {
  default     = "gcp-cpu-optimized"
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

variable "elasticsearch_dedicated_masters" {
  default     = false
  type        = bool
  description = "Optional Elasticsearch dedicated masters"
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

variable "apm_shards" {
  default     = 0
  type        = number
  description = "The number of shards to use for apm indices. Defaults to 0, doesn't modify the default values"
}

variable "drop_pipeline" {
  default     = false
  description = "Whether or not to install an Elasticsearch ingest pipeline to drop all incoming APM documents. Defaults to false"
  type        = bool
}

variable "apm_server_tail_sampling" {
  default     = false
  description = "Whether or not to enable APM Server tail-based sampling. Defaults to false"
  type        = bool
}

variable "apm_server_tail_sampling_storage_limit" {
  default     = "10GB"
  description = "Storage size limit of APM Server tail-based sampling. Defaults to 10GB"
  type        = string
}

variable "apm_server_tail_sampling_sample_rate" {
  default     = 0.1
  description = "Sample rate of APM Server tail-based sampling. Defaults to 0.1"
  type        = number

  validation {
    condition     = var.apm_server_tail_sampling_sample_rate >= 0 && var.apm_server_tail_sampling_sample_rate <= 1
    error_message = "The apm_server_tail_sampling_sample_rate value must be in range [0, 1]"
  }
}

# Standalone

variable "apm_server_bin_path" {
  default     = "../../build"
  type        = string
  description = "Optional path to APM Server binary"
}

variable "moxy_bin_path" {
  default     = "../../build"
  type        = string
  description = "Optional path to moxy binary"
}

variable "standalone_apm_server_instance_size" {
  default     = "c6i.2xlarge"
  type        = string
  description = "Optional instance type to use for APM Server VM"
}

variable "standalone_moxy_instance_size" {
  default     = "c6i.4xlarge"
  type        = string
  description = "Optional instance type to use for moxy VM"
}

variable "standalone_apm_server_volume_type" {
  default     = "gp3"
  type        = string
  description = "Optional volume type to use for APM Server VM"
}

variable "standalone_apm_server_volume_size" {
  default     = null
  type        = number
  description = "Optional volume size in GB to use for APM Server VM"
}

variable "standalone_apm_server_iops" {
  default     = null
  type        = number
  description = "Optional disk IOPS in GB to use for APM Server VM"
}

## VPC Network settings

variable "vpc_cidr" {
  default = "192.168.44.0/24"
  type    = string
}

variable "public_cidr" {
  default = [
    "192.168.44.0/26",
    "192.168.44.64/26",
    "192.168.44.128/26",
  ]
  type = list(string)
}

## Worker configuration

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
  default = "~/.ssh/id_rsa_terraform"
  type    = string
}

variable "public_key" {
  default = "~/.ssh/id_rsa_terraform.pub"
  type    = string
}

# CI variables
variable "BRANCH" {
  description = "Branch name or pull request for tagging purposes"
  default     = "unknown-branch"
}

variable "BUILD_ID" {
  description = "Build ID in the CI for tagging purposes"
  default     = "unknown-build"
}

variable "CREATED_DATE" {
  description = "Creation date in epoch time for tagging purposes"
  default     = ""
}

variable "ENVIRONMENT" {
  default = "unknown-environment"
}

variable "REPO" {
  default = "unknown-repo-name"
}
