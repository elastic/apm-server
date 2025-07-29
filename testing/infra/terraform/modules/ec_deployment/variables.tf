# Deployment settings

variable "deployment_name_prefix" {
  default     = "apmserver"
  description = "Optional ESS or ECE region. Defaults to GCP US West 2 (Los Angeles)"
  type        = string
}

variable "region" {
  default     = "gcp-us-west2"
  description = "Optional ESS or ECE region. Defaults to GCP US West 2 (Los Angeles)"
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

variable "observability_deployment" {
  default     = "none"
  type        = string
  description = "Optional deployment ID, for platform observability. Use 'self' to send data to the same deployment, 'dedicated' to create a dedicated observability deployment, or 'none' (the default) to disable observability. Any other value is assumed to be the ID of an existing, externally managed Elasticsearch deployment."
  validation {
    condition     = can(regex("^self$|^dedicated$|^none$", var.observability_deployment)) || length(var.observability_deployment) == 32
    error_message = "Invalid observability_deployment value. Must be one of 'self', 'dedicated', 'none', or '<deployment id>'."
  }
}

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Optional set of tags to use for all deployments"
}

# APM Server topology

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

variable "integrations_server" {
  description = "Optionally disable the integrations server block and use the apm block (7.x only)"
  type        = bool
  default     = true
}

# Elasticsearch topology

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

variable "elasticsearch_autoscale" {
  default     = false
  type        = bool
  description = "Optional autoscale the Elasticsearch cluster"
}

variable "elasticsearch_dedicated_masters" {
  default     = false
  type        = bool
  description = "Optionally use dedicated masters for the Elasticsearch cluster"
}

# Docker image overrides

variable "docker_image_tag_override" {
  default = {
    "elasticsearch" : "",
    "kibana" : "",
    "apm" : "",
  }
  description = "Optional docker image tag overrides, The full map needs to be specified"
  type        = map(string)
}

variable "docker_image" {
  default = {
    "elasticsearch" : "docker.elastic.co/cloud-release/elasticsearch-cloud-ess",
    "kibana" : "docker.elastic.co/cloud-release/kibana-cloud",
    "apm" : "docker.elastic.co/cloud-release/elastic-agent-cloud",
  }
  type        = map(string)
  description = "Optional docker image overrides. The full map needs to be specified"
}

# Enable APM Server's features

variable "apm_server_expvar" {
  default     = false
  description = "Whether or not to enable APM Server's expvar endpoint. Defaults to false"
  type        = bool
}

variable "apm_server_pprof" {
  default     = false
  description = "Whether or not to enable APM Server's pprof endpoint. Defaults to false"
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

variable "apm_index_shards" {
  default     = 0
  description = "The number of shards to set for APM Indices"
  type        = number
}

# Install custom APM integration package

variable "custom_apm_integration_pkg_path" {
  type        = string
  description = "Path to the zipped custom APM integration package, if empty custom apm integration pkg is not installed"
  default     = ""
}

# Install a custom pipeline to drop all the incoming APM documents. It can be
# useful to benchmark the theoretical APM Server output.
variable "drop_pipeline" {
  default     = false
  description = "Whether or not to install an Elasticsearch ingest pipeline to drop all incoming APM documents. Defaults to false"
  type        = bool
}
