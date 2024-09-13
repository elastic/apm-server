variable "instance_type" {
  type        = string
  description = "Moxy instance type"
}

variable "vpc_id" {
  description = "VPC ID to provision the EC2 instance"
  type        = string
}

variable "aws_provisioner_key_name" {
  description = "ssh key name to create the aws key pair and remote provision the EC2 instance"
  type        = string
}

variable "moxy_bin_path" {
  type        = string
  description = "Path to moxy binary from to copy to the worker machine"
}

variable "tags" {
  type        = map(string)
  default     = {}
  description = "Optional set of tags to use for all resources"
}
