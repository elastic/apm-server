variable "instance_type" {
  type        = string
  description = "Moxy instance type"
}

variable "moxy_bin_path" {
  type        = string
  description = "Optionally use the apm-server binary from the specified path to the worker machine"
}

variable "aws_provisioner_key_name" {
  default     = ""
  description = "Optional ssh key name to create the aws key pair and remote provision the ec2 instance"
  type        = string
}

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Optional set of tags to use for all resources"
}
