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
  apm_port  = "8200"
  conf_path = "/tmp/local-apm-config.yml"
  ea_provision_commands = [
    "curl ${data.external.latest_elastic_agent.result.deb} -o elastic-agent.deb && curl ${data.external.latest_elastic_agent.result.rpm} -o elastic-agent.rpm",
    "sudo dpkg -i elastic-agent.deb || sudo yum -y install elastic-agent.rpm",
    "sudo elastic-agent install -n",
    "sudo cp ${local.conf_path} /etc/elastic-agent/elastic-agent.yml",
    "sudo systemctl start elastic-agent",
    "sleep 1",
  ]
  standalone_provision_commands = [
    "curl ${data.external.latest_apm_server.result.deb} -o apm-server.deb && curl ${data.external.latest_apm_server.result.rpm} -o apm-server.rpm",
    "sudo dpkg -i apm-server.deb || sudo yum -y install apm-server.rpm",
    "sudo cp ${local.conf_path} /etc/apm-server/apm-server.yml",
    "sudo systemctl start apm-server",
    "sleep 1",
  ]
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

  provisioner "file" {
    destination = local.conf_path
    content = templatefile(var.ea_managed ? "${path.module}/elastic-agent.yml.tftpl" : "${path.module}/apm-server.yml.tftpl", {
      elasticsearch_url      = "${var.elasticsearch_url}",
      elasticsearch_username = "${var.elasticsearch_username}",
      elasticsearch_password = "${var.elasticsearch_password}",
      apm_version            = "${var.stack_version}"
      apm_secret_token       = "${random_password.apm_secret_token.result}"
      apm_port               = "${local.apm_port}"
    })
  }

  provisioner "remote-exec" {
    inline = concat(
      [for command in local.ea_provision_commands : command if var.ea_managed],
      [for command in local.standalone_provision_commands : command if !var.ea_managed],
    )
  }

  vpc_security_group_ids = [aws_security_group.main.id]
}

resource "null_resource" "apm_server_log" {
  triggers = {
    user        = local.image_ssh_users[var.aws_os]
    host        = aws_instance.apm.public_ip
    private_key = aws_key_pair.provisioner_key.key_name
  }

  depends_on = [aws_instance.apm]

  provisioner "local-exec" {
    when       = destroy
    command    = "scp -i ${self.triggers.private_key} -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null ${self.triggers.user}@${self.triggers.host}:/var/log/apm-server/apm-server-* ."
    on_failure = continue
  }
}

data "external" "latest_elastic_agent" {
  program = ["bash", "${path.module}/latest_elastic_agent.sh", "${var.stack_version}"]
}

data "external" "latest_apm_server" {
  program = ["bash", "${path.module}/latest_apm_server.sh", "${var.stack_version}"]
}

resource "aws_key_pair" "provisioner_key" {
  key_name   = var.aws_provisioner_key_name
  public_key = file("${var.aws_provisioner_key_name}.pub")
}

resource "random_password" "apm_secret_token" {
  length  = 16
  special = false
}
