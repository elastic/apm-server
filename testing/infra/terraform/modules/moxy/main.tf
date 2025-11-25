locals {
  moxy_port = "9200"
  bin_path  = "/tmp/moxy"

  # Detect if instance type is ARM (Graviton) based
  is_arm   = can(regex("^(a1|t4g|c6g|c7g|m6g|m7g|r6g|r7g|x2gd)", var.instance_type))
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

resource "aws_security_group" "main" {
  vpc_id = var.vpc_id
  egress = [
    {
      cidr_blocks      = ["0.0.0.0/0"]
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
      cidr_blocks      = ["0.0.0.0/0"]
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
      cidr_blocks      = ["0.0.0.0/0"]
      description      = ""
      from_port        = local.moxy_port
      ipv6_cidr_blocks = []
      prefix_list_ids  = []
      protocol         = "tcp"
      security_groups  = []
      self             = false
      to_port          = local.moxy_port
    }
  ]
}

resource "aws_instance" "moxy" {
  ami                    = data.aws_ami.worker_ami.id
  instance_type          = var.instance_type
  subnet_id              = data.aws_subnets.public_subnets.ids[0]
  vpc_security_group_ids = [aws_security_group.main.id]
  key_name               = aws_key_pair.provisioner_key.key_name
  monitoring             = false

  connection {
    type        = "ssh"
    user        = "ec2-user"
    host        = self.public_ip
    private_key = file("${var.aws_provisioner_key_name}")
  }

  provisioner "file" {
    source      = "${var.moxy_bin_path}/moxy"
    destination = local.bin_path
  }
  provisioner "remote-exec" {
    inline = [
      "sudo cp ${local.bin_path} moxy",
      "sudo chmod +x moxy",
      "nohup ./moxy -port=${local.moxy_port} -password=${random_password.moxy_password.result} > moxy.log 2>&1 &",
      "sleep 1"
    ]
  }

  tags = var.tags
}

resource "aws_key_pair" "provisioner_key" {
  public_key = file("${var.aws_provisioner_key_name}.pub")
  tags       = var.tags
}

resource "random_password" "moxy_password" {
  length  = 16
  special = false
}
