variable "ec_target" {
  type        = string
  description = "The Elastic Cloud environment to target"
  validation {
    condition     = contains(["qa", "pro"], var.ec_target)
    error_message = "Valid values are (qa, pro)."
  }
}

variable "ec_region" {
  type        = string
  description = "The Elastic Cloud region to target"
}

variable "ec_deployment_template" {
  type        = string
  description = "The deployment template to use. Must be available in the choosen region"
}

variable "name" {
  type        = string
  description = "The deployment name"
}

variable "stack_version" {
  type        = string
  description = "The Elasticsearch version to bootstrap"
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
