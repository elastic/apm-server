# Required

variable "user_name" {
  description = "Required username to use for resource name prefixes"
  type        = string
}

variable "apm_server_url" {
  description = "Required APM Server URL"
  type        = string
}

# Optional

variable "instance_type" {
  default     = "c6i.large"
  type        = string
  description = "Optional instance type to use for the worker VM"
}

variable "apm_secret_token" {
  default = ""
  type    = string
}

variable "public_key" {
  default = "~/.ssh/id_rsa_terraform.pub"
  type    = string
}

variable "private_key" {
  default = "~/.ssh/id_rsa_terraform"
  type    = string
}

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Optional set of tags to use for all resources"
}

## VPC Network settings

variable "vpc_cidr" {
  default = "192.168.44.0/24"
  type    = string
}

variable "public_cidr" {
  default = [
    "192.168.44.0/26",
    "192.168.44.64/26",
    "192.168.44.128/26",
  ]
  type = list(string)
}

variable "region" {
  default = "us-west2"
  type    = string
}

## APM Bench settings

variable "apmbench_bin_path" {
  default     = ""
  type        = string
  description = "Optionally upload the apmbench binary from the specified path to the worker machine"
}
