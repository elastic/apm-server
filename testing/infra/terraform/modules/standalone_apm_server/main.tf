locals {
  image_owners = {
    "ubuntu-focal-20.04-arm64-server" = "099720109477" # canonical
    "ubuntu-jammy-22.04-arm64-server" = "099720109477" # canonical
    "ubuntu-noble-24.04-arm64-server" = "099720109477" # canonical
    "debian-12-arm64"                 = "136693071363" # debian
    "al2023-ami-2023"                 = "137112412989" # amazon
    "al2023-ami-2023.*-x86_64"        = "137112412989" # amazon
    "RHEL-8"                          = "309956199498" # Red Hat
    "RHEL-9"                          = "309956199498" # Red Hat
    "Rocky-8-EC2-Base"                = "792107900819" # Rocky Linux
    "Rocky-9-EC2-Base"                = "792107900819" # Rocky Linux
    "AlmaLinux OS 8"                  = "764336703387" # AlmaLinux OS Foundation
    "AlmaLinux OS 9"                  = "764336703387" # AlmaLinux OS Foundation
    "OL8"                             = "131827586825" # Oracle
    "OL9"                             = "131827586825" # Oracle
  }
  instance_types = {
    "ubuntu-focal-20.04-arm64-server" = "t4g.nano"
    "ubuntu-jammy-22.04-arm64-server" = "t4g.nano"
    "ubuntu-noble-24.04-arm64-server" = "t4g.nano"
    "debian-12-arm64"                 = "t4g.nano"
    "al2023-ami-2023"                 = "t4g.nano"
    "al2023-ami-2023.*-x86_64"        = "t3a.micro"
    "RHEL-8"                          = "t4g.micro" # RHEL doesn't support nano instances
    "RHEL-9"                          = "t4g.micro" # RHEL doesn't support nano instances
    "Rocky-8-EC2-Base"                = "t4g.micro" # Larger instance size to improve test reliability
    "Rocky-9-EC2-Base"                = "t4g.nano"
    "AlmaLinux OS 8"                  = "t4g.nano"
    "AlmaLinux OS 9"                  = "t4g.nano"
    "OL8"                             = "t4g.nano"
    "OL9"                             = "t4g.nano"
  }
  instance_arch = {
    "ubuntu-focal-20.04-arm64-server" = "arm64"
    "ubuntu-jammy-22.04-arm64-server" = "arm64"
    "ubuntu-noble-24.04-arm64-server" = "arm64"
    "debian-12-arm64"                 = "arm64"
    "al2023-ami-2023"                 = "arm64"
    "al2023-ami-2023.*-x86_64"        = "x86_64"
    "RHEL-8"                          = "arm64"
    "RHEL-9"                          = "arm64"
    "Rocky-8-EC2-Base"                = "arm64"
    "Rocky-9-EC2-Base"                = "arm64"
    "AlmaLinux OS 8"                  = "arm64"
    "AlmaLinux OS 9"                  = "arm64"
    "OL8"                             = "arm64"
    "OL9"                             = "arm64"
  }
  instance_ea_provision_cmd = {
    "ubuntu-focal-20.04-arm64-server" = "curl ${data.external.latest_elastic_agent.result.deb_arm} -o elastic-agent.deb && sudo dpkg -i elastic-agent.deb"
    "ubuntu-jammy-22.04-arm64-server" = "curl ${data.external.latest_elastic_agent.result.deb_arm} -o elastic-agent.deb && sudo dpkg -i elastic-agent.deb"
    "ubuntu-noble-24.04-arm64-server" = "curl ${data.external.latest_elastic_agent.result.deb_arm} -o elastic-agent.deb && sudo dpkg -i elastic-agent.deb"
    "debian-12-arm64"                 = "curl ${data.external.latest_elastic_agent.result.deb_arm} -o elastic-agent.deb && sudo dpkg -i elastic-agent.deb"
    "al2023-ami-2023"                 = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "al2023-ami-2023.*-x86_64"        = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "RHEL-8"                          = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "RHEL-9"                          = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "Rocky-8-EC2-Base"                = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "Rocky-9-EC2-Base"                = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "AlmaLinux OS 8"                  = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "AlmaLinux OS 9"                  = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "OL8"                             = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
    "OL9"                             = "curl ${data.external.latest_elastic_agent.result.rpm_arm} -o elastic-agent.rpm && sudo rpm -i elastic-agent.rpm"
  }
  instance_standalone_provision_cmd = {
    "ubuntu-focal-20.04-arm64-server" = "curl ${data.external.latest_apm_server.result.deb_arm} -o apm-server.deb && sudo dpkg -i apm-server.deb"
    "ubuntu-jammy-22.04-arm64-server" = "curl ${data.external.latest_apm_server.result.deb_arm} -o apm-server.deb && sudo dpkg -i apm-server.deb"
    "ubuntu-noble-24.04-arm64-server" = "curl ${data.external.latest_apm_server.result.deb_arm} -o apm-server.deb && sudo dpkg -i apm-server.deb"
    "debian-12-arm64"                 = "curl ${data.external.latest_apm_server.result.deb_arm} -o apm-server.deb && sudo dpkg -i apm-server.deb"
    "al2023-ami-2023"                 = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "al2023-ami-2023.*-x86_64"        = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "RHEL-8"                          = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "RHEL-9"                          = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "Rocky-8-EC2-Base"                = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "Rocky-9-EC2-Base"                = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "AlmaLinux OS 8"                  = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "AlmaLinux OS 9"                  = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "OL8"                             = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
    "OL9"                             = "curl ${data.external.latest_apm_server.result.rpm_arm} -o apm-server.rpm && sudo rpm -i apm-server.rpm"
  }
  image_ssh_users = {
    "ubuntu-focal-20.04-arm64-server" = "ubuntu"
    "ubuntu-jammy-22.04-arm64-server" = "ubuntu"
    "ubuntu-noble-24.04-arm64-server" = "ubuntu"
    "debian-12-arm64"                 = "admin"
    "al2023-ami-2023"                 = "ec2-user"
    "al2023-ami-2023.*-x86_64"        = "ec2-user"
    "RHEL-8"                          = "ec2-user"
    "RHEL-9"                          = "ec2-user"
    "Rocky-8-EC2-Base"                = "rocky"
    "Rocky-9-EC2-Base"                = "rocky"
    "AlmaLinux OS 8"                  = "ec2-user"
    "AlmaLinux OS 9"                  = "ec2-user"
    "OL8"                             = "ec2-user"
    "OL9"                             = "ec2-user"
  }

  apm_port  = "8200"
  conf_path = "/tmp/local-apm-config.yml"
  bin_path  = "/tmp/apm-server"
}

data "aws_ami" "os" {
  most_recent = true

  filter {
    name   = "name"
    values = ["*${var.aws_os}*"]
  }

  filter {
    name   = "architecture"
    values = [local.instance_arch[var.aws_os]]
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

data "aws_region" "current" {}

data "aws_subnets" "public_subnets" {
  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
  filter {
    name   = "availability-zone"
    values = ["${data.aws_region.current.region}a"]
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
  ami                    = data.aws_ami.os.id
  instance_type          = var.apm_instance_type == "" ? local.instance_types[var.aws_os] : var.apm_instance_type
  subnet_id              = data.aws_subnets.public_subnets.ids[0]
  vpc_security_group_ids = [aws_security_group.main.id]
  key_name               = aws_key_pair.provisioner_key.key_name
  monitoring             = false

  root_block_device {
    volume_type = var.apm_volume_type
    volume_size = var.apm_volume_size
    iops        = var.apm_iops
  }

  connection {
    type        = "ssh"
    user        = local.image_ssh_users[var.aws_os]
    host        = self.public_ip
    private_key = file("${var.aws_provisioner_key_name}")
  }

  // For instance types with 'd.' e.g. c6id.2xlarge, use the NVMe ssd as data disk.
  provisioner "remote-exec" {
    inline = length(regexall("d[.]", self.instance_type)) > 0 ? [
      "sudo mkfs -t xfs /dev/nvme1n1",
      "mkdir ~/data",
      "sudo mount /dev/nvme1n1 ~/data",
      "sudo chown $USER:$USER ~/data",
      ] : [
      ":", // no-op
    ]
  }

  provisioner "file" {
    source      = "${var.apm_server_bin_path}/apm-server"
    destination = local.bin_path
    on_failure  = continue
  }

  provisioner "file" {
    destination = local.conf_path
    content = templatefile(var.ea_managed ? "${path.module}/elastic-agent.yml.tftpl" : "${path.module}/apm-server.yml.tftpl", {
      elasticsearch_url                      = "${var.elasticsearch_url}",
      elasticsearch_username                 = "${var.elasticsearch_username}",
      elasticsearch_password                 = "${var.elasticsearch_password}",
      apm_version                            = "${var.stack_version}"
      apm_secret_token                       = "${random_password.apm_secret_token.result}"
      apm_port                               = "${local.apm_port}"
      apm_server_tail_sampling               = "${var.apm_server_tail_sampling}"
      apm_server_tail_sampling_storage_limit = "${var.apm_server_tail_sampling_storage_limit}"
      apm_server_tail_sampling_sample_rate   = "${var.apm_server_tail_sampling_sample_rate}"
    })
  }

  provisioner "remote-exec" {
    inline = var.ea_managed ? [
      local.instance_ea_provision_cmd[var.aws_os],
      "sudo elastic-agent install -n --unprivileged",
      "sudo cp ${local.conf_path} /etc/elastic-agent/elastic-agent.yml",
      "sudo systemctl start elastic-agent",
      // oracle linux cloud image has firewalld enabled by default
      "sudo systemctl stop firewalld || true",
      "sleep 1",
      ] : (
      var.apm_server_bin_path == "" ? [
        local.instance_standalone_provision_cmd[var.aws_os],
        "sudo cp ${local.conf_path} /etc/apm-server/apm-server.yml",
        "sudo systemctl start apm-server",
        // oracle linux cloud image has firewalld enabled by default
        "sudo systemctl stop firewalld || true",
        "sleep 1",
        ] : [
        "sudo cp ${local.bin_path} apm-server",
        "sudo chmod +x apm-server",
        "sudo cp ${local.conf_path} apm-server.yml",
        "sudo mkdir -m 777 /var/log/apm-server",
        "screen -d -m ./apm-server",
        "sleep 1"
      ]
    )
  }

  tags = var.tags
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
  public_key = file("${var.aws_provisioner_key_name}.pub")
  tags       = var.tags
}

resource "random_password" "apm_secret_token" {
  length  = 16
  special = false
}
