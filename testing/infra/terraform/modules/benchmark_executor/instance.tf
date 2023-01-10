locals {
  ec2_tags = {
    name       = "${var.user_name}-worker"
    managed-by = "terraform"
    owner      = var.user_name
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "3.14.0"

  name = "${var.user_name}-worker"
  cidr = var.vpc_cidr

  azs                = [for letter in ["a", "b", "c"] : "${var.region}${letter}"]
  public_subnets     = var.public_cidr
  enable_ipv6        = false
  enable_nat_gateway = false
  single_nat_gateway = false

  manage_default_security_group = true
  default_security_group_ingress = [
    {
      "from_port" : 0,
      "to_port" : 0,
      "protocol" : -1,
      "self" : true,
      "cidr_blocks" : "0.0.0.0/0",
    }
  ]
  default_security_group_egress = [
    {
      "from_port" : 0,
      "to_port" : 0,
      "protocol" : -1,
      "cidr_blocks" : "0.0.0.0/0",
    }
  ]

  tags = merge(var.tags, local.ec2_tags)
  vpc_tags = {
    Name = "vpc-${var.user_name}-worker"
  }
}

resource "aws_key_pair" "worker" {
  key_name   = "${var.user_name}_worker_key"
  public_key = file(var.public_key)
  tags       = merge(var.tags, local.ec2_tags)
}

data "aws_ami" "worker_ami" {
  owners      = ["amazon"]
  most_recent = true

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-ebs"]
  }
}


module "ec2_instance" {
  source  = "terraform-aws-modules/ec2-instance/aws"
  version = "3.5.0"

  ami                         = data.aws_ami.worker_ami.id
  instance_type               = var.instance_type
  monitoring                  = false
  vpc_security_group_ids      = [module.vpc.default_security_group_id]
  subnet_id                   = module.vpc.public_subnets[0]
  associate_public_ip_address = true
  key_name                    = aws_key_pair.worker.id
  tags                        = merge(var.tags, local.ec2_tags)
}
