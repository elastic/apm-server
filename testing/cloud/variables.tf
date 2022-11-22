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
  default     = 1
  type        = number
  description = "Optional Elasticsearch zone count"
}

variable "observability_deployment" {
  default     = "self"
  type        = string
  description = "Optional deployment ID, for platform observability. Use 'self' (the default) to send data to the same deployment, 'dedicated' to create a dedicated observability deployment, or 'none' to disable observability. Any other value is assumed to be the ID of an existing, externally managed Elasticsearch deployment."
  validation {
    condition     = can(regex("^self$|^dedicated$|^none$", var.observability_deployment)) || length(var.observability_deployment) == 32
    error_message = "Invalid observability_deployment value. Must be one of 'self', 'dedicated', 'none', or '<deployment id>'."
  }
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
