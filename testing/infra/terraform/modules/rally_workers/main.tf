provider "google" {
  project = var.gcp_project
  region  = var.gcp_region
}

locals {
  ssh_user_name          = "rally"
  rally_corpora_zip_path = "${path.root}/build/corpora.zip"

  remote_python_bin_path        = "~/.local/bin"
  remote_working_dir            = "/home/rally"
  remote_rally_summary_file     = "rally-report.md"
  remote_ssh_connection_timeout = "500s"
}

module "tags" {
  source  = "../tags"
  project = "rallybenchmarks"
}

data "tls_public_key" "rally" {
  private_key_openssh = tls_private_key.rally.private_key_openssh
}

data "google_compute_image" "rally" {
  project = "ubuntu-os-cloud"
  family  = "ubuntu-minimal-2004-lts"
}

data "google_compute_zones" "available" {}

resource "tls_private_key" "rally" {
  algorithm = "RSA"
  rsa_bits  = "4096"
}

resource "google_compute_network" "rally-vpc" {
  name                    = "${var.resource_prefix}-rally-vpc"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "rally-subnet" {
  name          = "${var.resource_prefix}-rally-subnet"
  network       = google_compute_network.rally-vpc.self_link
  region        = var.gcp_region
  ip_cidr_range = var.rally_subnet_cidr
}

resource "google_compute_firewall" "rally-ssh" {
  name    = "${var.resource_prefix}-rally-ssh"
  network = google_compute_network.rally-vpc.self_link

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  source_ranges = ["0.0.0.0/0"]
}

resource "google_compute_firewall" "allow-internal" {
  name    = "${var.resource_prefix}-rally-allow-internal"
  network = google_compute_network.rally-vpc.self_link

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  source_ranges = [
    "${var.rally_subnet_cidr}"
  ]
}

resource "google_compute_instance" "rally_nodes" {
  # 1 rally node is for coordinator and others for distributing load generators
  count        = 1 + var.rally_worker_count
  name         = "${var.resource_prefix}-rally-nodes-${count.index}"
  machine_type = var.machine_type
  zone         = data.google_compute_zones.available.names[0]

  boot_disk {
    initialize_params {
      image = data.google_compute_image.rally.self_link
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.rally-subnet.self_link
    access_config {}
  }

  labels = module.tags.labels

  metadata = {
    ssh-keys = "${local.ssh_user_name}:${data.tls_public_key.rally.public_key_openssh}"
  }

  depends_on = [
    google_compute_firewall.rally-ssh,
  ]

  connection {
    type        = "ssh"
    host        = self.network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = [
      "sudo apt-get update",
      "sudo apt-get install -yq build-essential python3-pip unzip",
      "python3 -m pip install --quiet --no-warn-script-location --user --upgrade pip",
      "python3 -m pip install --quiet --no-warn-script-location --user esrally",
    ]
  }
}

resource "null_resource" "esrallyd_coordinator" {
  connection {
    type        = "ssh"
    host        = google_compute_instance.rally_nodes[0].network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = [
      <<EOF
${local.remote_python_bin_path}/esrallyd start \
  --node-ip=${google_compute_instance.rally_nodes[0].network_interface[0].network_ip} \
  --coordinator-ip=${google_compute_instance.rally_nodes[0].network_interface[0].network_ip}
      EOF
    ]
  }
}

resource "null_resource" "esrallyd_workers" {
  count = var.rally_worker_count
  triggers = {
    rally_nat_ip = element(
      [for i, n in google_compute_instance.rally_nodes : n.network_interface[0].access_config[0].nat_ip if i > 0],
      count.index,
    )
    rally_internal_ip = element(
      [for i, n in google_compute_instance.rally_nodes : n.network_interface[0].network_ip if i > 0],
      count.index,
    )
  }

  depends_on = [
    null_resource.esrallyd_coordinator,
  ]

  connection {
    type        = "ssh"
    host        = self.triggers.rally_nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = [
      <<EOF
${local.remote_python_bin_path}/esrallyd start \
  --node-ip=${self.triggers.rally_internal_ip} \
  --coordinator-ip=${google_compute_instance.rally_nodes[0].network_interface[0].network_ip}
      EOF
    ]
  }
}

data "archive_file" "corpora_zip" {
  type             = "zip"
  source_dir       = var.rally_dir
  output_file_mode = "0666"
  output_path      = local.rally_corpora_zip_path
}

resource "null_resource" "distribute_corpora" {
  # Corpora to be distributed to all workers and the coordinator
  count = var.rally_worker_count + 1
  triggers = {
    corpora_update = data.archive_file.corpora_zip.output_base64sha256
    rally_nat_ip = element(
      google_compute_instance.rally_nodes[*].network_interface[0].access_config[0].nat_ip,
      count.index,
    )
    rally_internal_ip = element(
      google_compute_instance.rally_nodes[*].network_interface[0].network_ip,
      count.index,
    )
  }

  connection {
    type        = "ssh"
    host        = self.triggers.rally_nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "file" {
    source      = local.rally_corpora_zip_path
    destination = "./corpora.zip"
  }

  provisioner "remote-exec" {
    inline = [
      "rm -rf ${local.remote_working_dir}/rally",
      "unzip -q -d ${local.remote_working_dir}/rally ${local.remote_working_dir}/corpora.zip",
      "rm ${local.remote_working_dir}/corpora.zip",
    ]
  }
}

resource "null_resource" "run_rally" {
  triggers = {
    always_run = "${timestamp()}"
  }

  depends_on = [
    null_resource.distribute_corpora
  ]

  connection {
    type        = "ssh"
    host        = google_compute_instance.rally_nodes[0].network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = [
      "rm -f ${local.remote_working_dir}/${local.remote_rally_summary_file}",
      <<EOF
${local.remote_python_bin_path}/esrally race \
  --target-hosts=${var.elasticsearch_url} \
  --load-driver-hosts=${join(",", google_compute_instance.rally_nodes[*].network_interface[0].network_ip)} \
  --client-options=use_ssl:true,basic_auth_user:${var.elasticsearch_username},basic_auth_password:${var.elasticsearch_password} \
  --track-path=${local.remote_working_dir}/rally \
  --track-params=expected_cluster_health:${var.rally_cluster_status},bulk_size:${var.rally_bulk_size},bulk_clients:${var.rally_bulk_clients} \
  --kill-running-process \
  --pipeline=benchmark-only \
  --report-file=${local.remote_working_dir}/${local.remote_rally_summary_file}
      EOF
    ]
  }
}

data "remote_file" "rally_summary" {
  conn {
    host        = google_compute_instance.rally_nodes[0].network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    private_key = tls_private_key.rally.private_key_openssh
  }

  depends_on = [
    null_resource.run_rally,
  ]

  path = "${local.remote_working_dir}/${local.remote_rally_summary_file}"
}
