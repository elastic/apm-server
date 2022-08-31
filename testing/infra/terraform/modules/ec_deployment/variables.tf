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
  default     = "gcp-compute-optimized-v2"
  description = "Optional deployment template. Defaults to the CPU optimized template for GCP"
  type        = string
}

variable "stack_version" {
  default     = "latest"
  description = "Optional stack version"
  type        = string
}

variable "monitor_deployment" {
  default     = false
  type        = bool
  description = "Optionally monitor the deployment in a separate deployment"
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
  description = "Optionally use the integrations server block instead of the apm block"
  type        = bool
  default     = false
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

# Enable APM Server's expvar

variable "apm_server_expvar" {
  default     = true
  description = "Whether or not to enable APM Server's expvar endpoint. Defaults to true"
  type        = bool
}

variable "apm_server_pprof" {
  default     = true
  description = "Whether or not to enable APM Server's pprof endpoint. Defaults to true"
  type        = bool
}

variable "apm_index_shards" {
  default     = 0
  description = "The number of shards to set for APM Indices"
  type        = number
}
