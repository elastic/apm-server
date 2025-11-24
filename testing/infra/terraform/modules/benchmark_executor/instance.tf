locals {
  ec2_tags = {
    name       = "${var.user_name}-worker"
    managed-by = "terraform"
    owner      = var.user_name
  }

  # Detect if instance type is ARM (Graviton) based
  is_arm = can(regex("^(a1|t4g|c6g|c7g|m6g|m7g|r6g|r7g|x2gd)", var.instance_type))
  ami_arch = local.is_arm ? "arm64" : "x86_64"
}

data "aws_ami" "worker_ami" {
  owners      = ["amazon"]
  most_recent = true

  filter {
    name   = "name"
    values = ["al2023-ami-*-${local.ami_arch}"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

data "aws_subnets" "public_subnets" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
}

data "aws_security_group" "security_group" {
  vpc_id = var.vpc_id
  name   = "default"
}


module "ec2_instance" {
  source  = "terraform-aws-modules/ec2-instance/aws"
  version = "3.5.0"

  ami                         = data.aws_ami.worker_ami.id
  instance_type               = var.instance_type
  monitoring                  = false
  vpc_security_group_ids      = [data.aws_security_group.security_group.id]
  subnet_id                   = data.aws_subnets.public_subnets.ids[0]
  associate_public_ip_address = true
  key_name                    = aws_key_pair.worker.id
  tags                        = merge(var.tags, local.ec2_tags)
}

resource "aws_key_pair" "worker" {
  key_name   = "${var.user_name}_worker_key"
  public_key = file(var.public_key)
  tags       = merge(var.tags, local.ec2_tags)
}
