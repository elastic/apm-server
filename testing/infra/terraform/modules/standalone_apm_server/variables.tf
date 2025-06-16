variable "aws_os" {
  default     = ""
  description = "Optional aws EC2 instance OS"
  type        = string
}

variable "apm_instance_type" {
  default     = ""
  type        = string
  description = "Optional apm server instance type override"
}

variable "apm_volume_type" {
  default     = null
  type        = string
  description = "Optional apm server volume type override"
}

variable "apm_volume_size" {
  default     = null
  type        = number
  description = "Optional apm server volume size in GB override"
}

variable "apm_iops" {
  default     = null
  type        = number
  description = "Optional apm server disk IOPS override"
}

variable "vpc_id" {
  description = "VPC ID to provision the EC2 instance"
  type        = string
}

variable "aws_provisioner_key_name" {
  description = "ssh key name to create the aws key pair and remote provision the EC2 instance"
  type        = string
}

variable "elasticsearch_url" {
  description = "The secure Elasticsearch URL"
  type        = string
}

variable "elasticsearch_username" {
  sensitive   = true
  type        = string
  description = "The Elasticsearch username"
}

variable "elasticsearch_password" {
  sensitive   = true
  type        = string
  description = "The Elasticsearch password"
}

variable "stack_version" {
  default     = "latest"
  description = "Optional stack version"
  type        = string
}

variable "ea_managed" {
  default     = false
  description = "Whether or not install Elastic Agent managed APM Server"
  type        = bool
}

variable "apm_server_bin_path" {
  default     = ""
  type        = string
  description = "Optionally use the apm-server binary from the specified path instead"
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

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Optional set of tags to use for all deployments"
}
