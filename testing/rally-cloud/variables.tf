variable "ess_region" {
  type        = string
  description = "Optional ESS region where the deployment will be created. Defaults to gcp-us-west2"
  default     = "gcp-us-west2"
}

variable "deployment_template" {
  type        = string
  description = "Optional deployment template. Defaults to the CPU optimized template for GCP"
  default     = "gcp-compute-optimized-v2"
}

variable "stack_version" {
  type        = string
  description = "Optional stack version"
  default     = "latest"
}

variable "elasticsearch_size" {
  type        = string
  description = "Optional Elasticsearch instance size"
  default     = "8g"
}

variable "elasticsearch_zone_count" {
  type        = number
  description = "Optional Elasticsearch zone count"
  default     = 1
}

variable "custom_apm_integration_pkg_path" {
  type        = string
  description = "Path to the zipped custom APM integration package, if empty custom apm integration pkg is not installed"
}

variable "gcp_project" {
  type        = string
  description = "GCP Project name"
  default     = "elastic-apm"
}

variable "gcp_region" {
  type        = string
  description = "GCP region"
  default     = "us-west2"
}

variable "rally_machine_type" {
  type        = string
  description = "Machine type for rally nodes"
  default     = "e2-small"
}

variable "rally_workers_resource_prefix" {
  type        = string
  description = "Prefix to add to created resource for rally workers"
}

variable "rally_dir" {
  type        = string
  description = "Path to the directory containing track and corpora files"
  default     = "../../testing/rally"
}

variable "rally_bulk_size" {
  type        = number
  description = "Bulk size to use for rally track"
  default     = 5000
}

variable "rally_cluster_status" {
  type        = string
  description = "Expected cluster status for rally"
  default     = "green"
}

variable "rally_worker_count" {
  type        = string
  description = "Number of rally worker nodes"
  default     = 2
}

variable "rally_bulk_clients" {
  type        = number
  description = "Number of clients to use for rally bulk requests"
  default     = 10
}
