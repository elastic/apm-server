terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    ec = {
      source  = "elastic/ec"
      version = "0.5.1"
    }
  }
}

provider "ec" {}

module "ec_deployment" {
  source = "../../infra/terraform/modules/ec_deployment"
  region = var.region

  deployment_template    = "gcp-compute-optimized-v2"
  deployment_name_prefix = "smoke-upgrade"

  apm_server_size = "1g"

  elasticsearch_size       = "4g"
  elasticsearch_zone_count = 1

  stack_version       = var.stack_version
}

locals {
  image_owners = {
    "ubuntu-bionic-18.04-amd64-server" = "099720109477" # canonical
    "ubuntu-focal-20.04-amd64-server"  = "099720109477" # canonical
    "ubuntu-jammy-22.04-amd64-server"  = "099720109477" # canonical
    "debian-10-amd64"                  = "136693071363" # debian
    "debian-11-amd64"                  = "136693071363" # debian
    "amzn2-ami-kernel-5.10"            = "137112412989" # amazon
    "RHEL-7"                           = "309956199498" # Red Hat
    "RHEL-8"                           = "309956199498" # Red Hat
    "RHEL-9"                           = "309956199498" # Red Hat
  }
  image_ssh_users = {
    "ubuntu-bionic-18.04-amd64-server" = "ubuntu"
    "ubuntu-focal-20.04-amd64-server"  = "ubuntu"
    "ubuntu-jammy-22.04-amd64-server"  = "ubuntu"
    "debian-10-amd64"                  = "admin"
    "debian-11-amd64"                  = "admin"
    "amzn2-ami-kernel-5.10"            = "ec2-user"
    "RHEL-7"                           = "ec2-user"
    "RHEL-8"                           = "ec2-user"
    "RHEL-9"                           = "ec2-user"
  }
  apm_port = "8200"
}

data "aws_ami" "os" {
  most_recent = true

  filter {
    name   = "name"
    values = ["*${var.aws_os}*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = [local.image_owners[var.aws_os]]
}

resource "aws_security_group" "main" {
  egress = [
    {
      cidr_blocks      = ["0.0.0.0/0", ]
      description      = ""
      from_port        = 0
      ipv6_cidr_blocks = []
      prefix_list_ids  = []
      protocol         = "-1"
      security_groups  = []
      self             = false
      to_port          = 0
    }
  ]
  ingress = [
    {
      cidr_blocks      = ["0.0.0.0/0", ]
      description      = ""
      from_port        = 22
      ipv6_cidr_blocks = []
      prefix_list_ids  = []
      protocol         = "tcp"
      security_groups  = []
      self             = false
      to_port          = 22
    },
    {
      cidr_blocks      = ["0.0.0.0/0", ]
      description      = ""
      from_port        = local.apm_port
      ipv6_cidr_blocks = []
      prefix_list_ids  = []
      protocol         = "tcp"
      security_groups  = []
      self             = false
      to_port          = local.apm_port
    }
  ]
}

resource "aws_instance" "apm" {
  ami           = data.aws_ami.os.id
  instance_type = "t3.micro"
  key_name      = aws_key_pair.provisioner_key.key_name

  connection {
    type        = "ssh"
    user        = local.image_ssh_users[var.aws_os]
    host        = self.public_ip
    private_key = file("${var.aws_provisioner_key_name}")
  }

  provisioner "remote-exec" {
    inline = [
      "curl ${data.external.getlatestapmserver.result.deb} -o apm-server.deb && curl ${data.external.getlatestapmserver.result.rpm} -o apm-server.rpm",
      "sudo dpkg -i apm-server.deb || sudo yum -y install apm-server.rpm",
      "sudo rm /etc/apm-server/apm-server.yml",
      "echo \"apm-server:\n  host: \\\"0.0.0.0:${local.apm_port}\\\"\noutput:\n  elasticsearch:\n    hosts: [\\\"${module.ec_deployment.elasticsearch_url}\\\"]\n    username: \\\"${module.ec_deployment.elasticsearch_username}\\\"\n    password: \\\"${module.ec_deployment.elasticsearch_password}\\\"\" | sudo tee -a /etc/apm-server/apm-server.yml",
      "sudo systemctl start apm-server",
      "sleep 1",
    ]
  }

  vpc_security_group_ids = [aws_security_group.main.id]
}

data "external" "getlatestapmserver" {
  program = ["sh", "./latest_apm_server.sh"]
}

resource "aws_key_pair" "provisioner_key" {
  key_name   = var.aws_provisioner_key_name
  public_key = file("${var.aws_provisioner_key_name}.pub")
}

resource "random_password" "apm_secret_token" {
  length  = 16
  special = false
}

variable "aws_os" {
  default = ""
  description = "Optional aws ec2 instance OS"
  type    = string
}

variable "aws_provisioner_key_name" {
  default = ""
  description = "Optional ssh key name to create the aws key pair and remote provision the ec2 instance"
  type    = string
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

output "apm_secret_token" {
  value       = random_password.apm_secret_token.result
  description = "The APM Server secret token"
  sensitive   = true
}

output "apm_server_url" {
  value       = "${aws_instance.apm.public_ip}:${local.apm_port}"
  description = "The APM Server URL"
}

output "kibana_url" {
  value       = module.ec_deployment.kibana_url
  description = "The Kibana URL"
}

output "elasticsearch_url" {
  value       = module.ec_deployment.elasticsearch_url
  description = "The Elasticsearch URL"
}

output "elasticsearch_username" {
  value       = module.ec_deployment.elasticsearch_username
  sensitive   = true
  description = "The Elasticsearch username"
}

output "elasticsearch_password" {
  value       = module.ec_deployment.elasticsearch_password
  sensitive   = true
  description = "The Elasticsearch password"
}

output "stack_version" {
  value       = module.ec_deployment.stack_version
  description = "The matching stack pack version from the provided stack_version"
}
