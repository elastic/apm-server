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

variable "gcp_zone" {
  type        = string
  description = "GCP zone"
  default     = "us-west2-b"
}

variable "machine_type" {
  type        = string
  description = "Machine type for soak test workers"
  default     = "e2-small"
}

variable "apmsoak_bin_path" {
  type        = string
  description = "Path where the apmsoak binary resides on the local machine"
}

variable "apm_server_url" {
  type        = string
  description = "APM Server URL for sending the generated load"
}

variable "apm_secret_token" {
  type        = string
  description = "Secret token for auth against the given server URL"
  sensitive   = true
}

variable "apm_loadgen_event_rate" {
  type        = string
  description = "Load generation rate"
}

variable "apm_loadgen_agents_replicas" {
  type        = string
  description = "Number of agents replicas to use, each replica launches 4 agents, one for each type"
}

variable "apm_loadgen_rewrite_timestamps" {
  type        = bool
  description = "Rewrite event timestamps every iteration, maintaining relative offsets"
  default     = false
}

variable "apm_loadgen_rewrite_ids" {
  type        = bool
  description = "Rewrite event IDs every iteration, maintaining event relationships"
  default     = false
}

variable "elastic_agent_version" {
  type        = string
  description = "Version of elastic-agent to install on the monitoring worker nodes"
}

variable "fleet_url" {
  type        = string
  description = "Fleet URL to enroll elastic agent for monitoring worker nodes"
}

variable "fleet_enrollment_token" {
  type        = string
  description = "Fleet enrollment token to enroll elastic agent for monitoring worker nodes"
}
