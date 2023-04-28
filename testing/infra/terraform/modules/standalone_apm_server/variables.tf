variable "aws_os" {
  default     = ""
  description = "Optional aws ec2 instance OS"
  type        = string
}

variable "aws_provisioner_key_name" {
  default     = ""
  description = "Optional ssh key name to create the aws key pair and remote provision the ec2 instance"
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

variable "worker_region" {
  default     = "us-west-2"
  description = "Optional AWS region where the workers will be created. Defaults to us-west-2 (AWS)"
  type        = string
}

variable "ea_managed" {
  default     = false
  description = "Whether or not install Elastic Agent managed APM Server"
  type        = bool
}

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Optional set of tags to use for all deployments"
}

# CI variables
variable "BRANCH" {
  description = "Branch name or pull request for tagging purposes"
  default     = "unknown"
}

variable "BUILD_ID" {
  description = "Build ID in the CI for tagging purposes"
  default     = "unknown"
}

variable "CREATED_DATE" {
  description = "Creation date in epoch time for tagging purposes"
  default     = "unknown"
}

variable "ENVIRONMENT" {
  default = "unknown"
}

variable "REPO" {
  default = "unknown"
}
