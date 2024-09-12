locals {
  moxy_port = "9200"
  bin_path  = "/tmp/moxy"
}

data "aws_ami" "worker_ami" {
  owners      = ["amazon"]
  most_recent = true

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-ebs"]
  }
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
  ami           = data.aws_ami.worker_ami.id
  instance_type = var.instance_type
  monitoring    = false
  key_name      = aws_key_pair.provisioner_key.key_name

  connection {
    type        = "ssh"
    user        = "ec2-user"
    host        = self.public_ip
    private_key = file("${var.aws_provisioner_key_name}")
  }

  provisioner "file" {
    source      = var.moxy_bin_path
    destination = local.bin_path
  }
  provisioner "remote-exec" {
    inline = [
      "sudo cp ${local.bin_path} moxy",
      "chmod +x moxy",
      "./moxy &"
    ]
  }

  vpc_security_group_ids = [aws_security_group.main.id]

  tags = var.tags
}

resource "aws_key_pair" "provisioner_key" {
  public_key = file("${var.aws_provisioner_key_name}.pub")
  tags       = var.tags
}
