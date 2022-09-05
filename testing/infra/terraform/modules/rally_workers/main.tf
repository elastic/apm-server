provider "google" {
  project = var.gcp_project
  region  = var.gcp_region
}

locals {
  ssh_user_name             = "rally"
  python_bin_path           = "~/.local/bin"
  remote_rally_summary_file = "rally-report.md"
  remote_ssh_connection_timeout = "500s"
  esrally_install_cmd = [
    "sudo apt-get update",
    "sudo apt-get install -yq build-essential python3-pip git bzip2 pigz",
    "python3 -m pip install --quiet --no-warn-script-location --user --upgrade pip",
    "python3 -m pip install --quiet --no-warn-script-location --user esrally",
  ]
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

resource "google_compute_network" "rally" {
  name = "${var.resource_prefix}-rally-network"
}

resource "google_compute_firewall" "rally" {
  name    = "${var.resource_prefix}-rally-firewall"
  network = google_compute_network.rally.self_link

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  source_ranges = ["0.0.0.0/0"]
}

resource "google_compute_instance" "rally_coordinator" {
  name         = "${var.resource_prefix}-rally-coordinator"
  machine_type = "e2-micro"
  zone         = data.google_compute_zones.available.names[0]

  boot_disk {
    initialize_params {
      image = data.google_compute_image.rally.self_link
    }
  }

  network_interface {
    network = google_compute_network.rally.self_link
    access_config {}
  }

  labels = {
    team    = "apm-server"
    purpose = "local-rally-test"
  }

  metadata = {
    ssh-keys = "${local.ssh_user_name}:${data.tls_public_key.rally.public_key_openssh}"
  }

  depends_on = [
    google_compute_firewall.rally,
  ]

  connection {
    type        = "ssh"
    host        = self.network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout 
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = concat(
      local.esrally_install_cmd,
      [
        "${local.python_bin_path}/esrallyd start --node-ip=${self.network_interface.0.network_ip} --coordinator-ip=${self.network_interface.0.network_ip}",
      ]
    )
  }
}

resource "google_compute_instance" "rally_workers" {
  count        = var.rally_worker_count
  name         = "${var.resource_prefix}-rally-worker-${count.index}"
  machine_type = "e2-micro"
  zone         = data.google_compute_zones.available.names[0]

  boot_disk {
    initialize_params {
      image = data.google_compute_image.rally.self_link
    }
  }

  network_interface {
    network = google_compute_network.rally.self_link
    access_config {}
  }

  labels = {
    team    = "apm-server"
    purpose = "local-rally-test"
  }

  metadata = {
    ssh-keys = "${local.ssh_user_name}:${data.tls_public_key.rally.public_key_openssh}"
  }

  depends_on = [
    google_compute_firewall.rally,
  ]

  connection {
    type        = "ssh"
    host        = self.network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout 
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = concat(
      local.esrally_install_cmd,
      [
        "${local.python_bin_path}/esrallyd start --node-ip=${self.network_interface.0.network_ip} --coordinator-ip=${google_compute_instance.rally_coordinator.network_interface.0.network_ip}",
      ]
    )
  }
}

resource "null_resource" "run_rally" {
  triggers = {
    always_run = "${timestamp()}"
  }

  depends_on = [
    google_compute_instance.rally_workers
  ]

  connection {
    type        = "ssh"
    host        = google_compute_instance.rally_coordinator.network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    timeout     = local.remote_ssh_connection_timeout 
    private_key = tls_private_key.rally.private_key_openssh
  }

  provisioner "file" {
    source      = var.rally_dir
    destination = "./"
  }

  provisioner "remote-exec" {
    inline = [
      "rm ${local.remote_rally_summary_file}",
      "${local.python_bin_path}/esrally race --target-hosts=${var.elasticsearch_url} --client-options=use_ssl:true,basic_auth_user:${var.elasticsearch_username},basic_auth_password:${var.elasticsearch_password} --track-path=rally --track-params=expected_cluster_health:${var.rally_cluster_status},bulk_size:${var.rally_bulk_size} --kill-running-process --pipeline=benchmark-only --report-file=${local.remote_rally_summary_file}"
    ]
  }
}

data "remote_file" "rally_summary" {
  conn {
    host        = google_compute_instance.rally_coordinator.network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    private_key = tls_private_key.rally.private_key_openssh
  }

  depends_on = [
    null_resource.run_rally,
  ]

  path = local.remote_rally_summary_file
}
