locals {
  ssh_user_name      = "apmsoak"
  executable_name    = "apmsoak"
  service_name       = "apmsoak.service"
  remote_working_dir = "apmsoak/"
  config_hash = sha256(join("_",
    [
      terraform.workspace,
      var.apm_server_url,
      var.apm_secret_token,
      var.apm_loadgen_event_rate,
      var.apm_loadgen_agents_replicas,
      var.elastic_agent_version,
      var.fleet_url,
    ]
  ))
}

module "tags" {
  source  = "../tags"
  project = "soaktests"
}

resource "tls_private_key" "worker_login" {
  algorithm = "RSA"
  rsa_bits  = "4096"
}

data "tls_public_key" "worker_login" {
  private_key_openssh = tls_private_key.worker_login.private_key_openssh
}

resource "google_compute_network" "worker" {
  name = "apmsoak-network-${terraform.workspace}"
}

resource "google_service_account" "worker" {
  account_id   = "apmsoak-${terraform.workspace}"
  display_name = "APM Server soaktest worker"
}

resource "google_compute_firewall" "allow_ssh" {
  name    = "apmsoak-allowssh-${terraform.workspace}"
  network = google_compute_network.worker.self_link

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  source_ranges = ["0.0.0.0/0"]
}

data "google_compute_image" "worker_image" {
  project = "ubuntu-os-cloud"
  family  = "ubuntu-minimal-2404-lts-amd64"
}

resource "google_compute_instance" "worker" {
  # Trigger a recreate of workers on config changes
  name         = substr("apmsoak-${local.config_hash}", 0, 63)
  machine_type = var.machine_type
  zone         = var.gcp_zone

  boot_disk {
    initialize_params {
      image = data.google_compute_image.worker_image.self_link
    }
  }

  network_interface {
    network = google_compute_network.worker.self_link
    access_config {}
  }

  metadata = {
    ssh-keys = "${local.ssh_user_name}:${data.tls_public_key.worker_login.public_key_openssh}"
  }

  service_account {
    email  = google_service_account.worker.email
    scopes = ["cloud-platform"]
  }

  depends_on = [
    google_compute_firewall.allow_ssh,
  ]

  connection {
    type        = "ssh"
    host        = self.network_interface[0].access_config[0].nat_ip
    user        = local.ssh_user_name
    timeout     = "500s"
    private_key = tls_private_key.worker_login.private_key_openssh
  }

  provisioner "remote-exec" {
    inline = [
      "mkdir -p ${local.remote_working_dir}",
      # Install elastic-agent for monitoring
      "cd ${local.remote_working_dir}",
      "curl -s -L -O https://artifacts.elastic.co/downloads/beats/elastic-agent/elastic-agent-${var.elastic_agent_version}-linux-x86_64.tar.gz",
      "tar xzvf elastic-agent-${var.elastic_agent_version}-linux-x86_64.tar.gz",
      "cd elastic-agent-${var.elastic_agent_version}-linux-x86_64",
      "sudo ./elastic-agent install -n --url=${var.fleet_url} --enrollment-token=${var.fleet_enrollment_token}",
    ]
  }

  provisioner "file" {
    source      = "${var.apmsoak_bin_path}/${local.executable_name}"
    destination = "${local.remote_working_dir}/${local.executable_name}"
  }

  provisioner "file" {
    content = templatefile(
      "${path.module}/systemd.tpl",
      {
        apmsoak_executable_path = "/bin/apmsoak",
        remote_user             = "apmsoak",
        remote_usergroup        = "apmsoak",
        apm_server_url          = var.apm_server_url,
        apm_secret_token        = var.apm_secret_token,
        scenarios_yml           = "${local.remote_working_dir}/scenarios.yml"
      }
    )
    destination = "${local.remote_working_dir}/${local.service_name}"
  }

  provisioner "file" {
    content = templatefile(
      "${path.module}/scenarios.tpl",
      {
        apm_loadgen_event_rate         = var.apm_loadgen_event_rate,
        apm_loadgen_agents_replicas    = var.apm_loadgen_agents_replicas,
        apm_loadgen_rewrite_timestamps = var.apm_loadgen_rewrite_timestamps
        apm_loadgen_rewrite_ids        = var.apm_loadgen_rewrite_ids
      }
    )
    destination = "${local.remote_working_dir}/scenarios.yml"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x ${local.remote_working_dir}/${local.executable_name}",
      "sudo mv ${local.remote_working_dir}/${local.executable_name} /bin/",
      "sudo mv ${local.remote_working_dir}/${local.service_name} /lib/systemd/system/.",
      "sudo chmod 755 /lib/systemd/system/${local.service_name}",
      "sudo systemctl enable ${local.service_name}",
      "sudo systemctl start ${local.service_name}",
    ]
  }

}
