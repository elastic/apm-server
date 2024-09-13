variable "aws_os" {
  default     = ""
  description = "Optional aws EC2 instance OS"
  type        = string
}

variable "apm_instance_type" {
  default     = ""
  type        = string
  description = "Optional apm server instance type overide"
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

variable "region" {
  default     = "gcp-us-west2"
  description = "Optional ESS region where to run the smoke tests"
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

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Optional set of tags to use for all deployments"
}
