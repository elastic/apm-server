## General configuration

variable "user_name" {
  description = "Required username to use for prefixes"
  type        = string
}

## Deployment configuration

variable "apm_instance_type" {
  default     = "c6i.large"
  type        = string
  description = "Optional apm server instance type"
}

variable "apm_server_bin_path" {
  default     = "../../build/apm-server-linux-amd64"
  type        = string
  description = "Optional path to the apm-server binary"
}

variable "moxy_instance_type" {
  default     = "c6i.large"
  type        = string
  description = "Optional moxy instance type"
}

variable "moxy_bin_path" {
  default     = "../../systemtest/cmd/moxy"
  type        = string
  description = "Optional path to the moxy binary"
}

## Worker configuraiton

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
