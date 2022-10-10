variable "resource_prefix" {
  type        = string
  description = "Prefix to add to all created resource"
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

variable "machine_type" {
  type        = string
  description = "Machine type for rally nodes"
  default     = "e2-small"
}

variable "elasticsearch_url" {
  type        = string
  description = "Elasticsearch URL to benchmark with rally"
}

variable "elasticsearch_username" {
  type        = string
  description = "Elasticsearch username to use for benchmark with rally"
}

variable "elasticsearch_password" {
  type        = string
  description = "Elasticsearch password to use for benchmark with rally"
}

variable "rally_subnet_cidr" {
  type        = string
  description = "CIDR block for subnet containing rally instances"
  default     = "10.128.0.0/20"
}

variable "rally_worker_count" {
  type        = number
  description = "Number of rally worker nodes"
  default     = 2
}

variable "rally_dir" {
  type        = string
  description = "Directory path with rally corpora and track file"
}

variable "rally_cluster_status" {
  type        = string
  description = "Expected cluster status for rally"
  default     = "green"
}

variable "rally_bulk_size" {
  type        = number
  description = "Bulk size to use for rally track"
  default     = 5000
}

variable "rally_bulk_clients" {
  type        = number
  description = "Number of clients to use for rally bulk requests"
  default     = 10
}
