locals {
  user = "ec2-user"
}

resource "null_resource" "apmbench" {
  count = var.apmbench_bin_path != "" ? 1 : 0

  triggers = {
    apmbench  = filesha1("${var.apmbench_bin_path}/apmbench")
    worker_ip = module.ec2_instance.public_ip
  }

  connection {
    host        = module.ec2_instance.public_ip
    user        = local.user
    private_key = file(var.private_key)
  }

  provisioner "remote-exec" {
    inline = ["mkdir -p bin"]
  }
  provisioner "file" {
    source      = "${var.apmbench_bin_path}/apmbench"
    destination = "bin/apmbench"
  }

  provisioner "remote-exec" {
    inline = ["chmod +x ~/bin/apmbench"]
  }
}

resource "null_resource" "envrc" {
  depends_on = [local_file.envrc]

  triggers = {
    worker_ip = module.ec2_instance.public_ip
    envrc     = local_file.envrc.id
  }

  connection {
    host        = module.ec2_instance.public_ip
    user        = local.user
    private_key = file(var.private_key)
  }

  provisioner "file" {
    source      = ".envrc"
    destination = ".envrc"
  }

  provisioner "remote-exec" {
    inline = ["grep .envrc .bashrc || echo '. .envrc' >> .bashrc"]
  }
}

resource "local_file" "envrc" {
  content  = <<EOT
export ELASTIC_APM_SECRET_TOKEN="${var.apm_secret_token}"
export ELASTIC_APM_SERVICE_NAME="apmbench"
export ELASTIC_APM_SERVER_URL="${var.apm_server_url}"
export ELASTIC_APM_VERIFY_SERVER_CERT="false"

# Set the service environment
export ELASTIC_APM_ENVIRONMENT="bench"
export ELASTIC_APM_BREAKDOWN_METRICS="true"
export ELASTIC_APM_CAPTURE_HEADERS="true"
export ELASTIC_APM_CAPTURE_BODY="all"

# OTEL credentials
export OTEL_RESOURCE_ATTRIBUTES=service.name=apmbench,deployment.environment=bench
export OTEL_EXPORTER_OTLP_ENDPOINT="${replace(var.apm_server_url, "https://", "")}"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer $${ELASTIC_APM_SECRET_TOKEN}"
EOT
  filename = ".envrc"
}
