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
  description = "Moxy path to binary to copy to the worker machine"
}
