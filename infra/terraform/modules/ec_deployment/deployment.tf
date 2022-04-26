locals {
  region              = var.region
  version             = var.stack_version
  deployment_template = var.deployment_template
  name_prefix         = var.deployment_name_prefix
}

data "ec_stack" "deployment_version" {
  version_regex = local.version
  region        = local.region
}

resource "ec_deployment" "deployment" {
  name                   = "${local.name_prefix}-${local.version}"
  version                = data.ec_stack.deployment_version.version
  region                 = local.region
  deployment_template_id = local.deployment_template

  elasticsearch {
    topology {
      id         = "hot_content"
      size       = var.elasticsearch_size
      zone_count = var.elasticsearch_zone_count
    }
    dynamic "config" {
      for_each = var.docker_image_tag_override != "" ? [var.docker_image["elasticsearch"]] : []
      content {
        docker_image = "${config.value}:${var.docker_image_tag_override}"
      }
    }
  }
  kibana {
    dynamic "config" {
      for_each = var.docker_image_tag_override != "" ? [var.docker_image["kibana"]] : []
      content {
        docker_image = "${config.value}:${var.docker_image_tag_override}"
      }
    }
  }
  apm {
    dynamic "config" {
      for_each = var.docker_image_tag_override != "" ? [var.docker_image["apm"]] : []
      content {
        docker_image = "${config.value}:${var.docker_image_tag_override}"
      }
    }
    topology {
      size       = var.apm_server_size
      zone_count = var.apm_server_zone_count
    }
  }

  dynamic "observability" {
    for_each = ec_deployment.deployment_monitor.*
    content {
      deployment_id = observability.value.id
      logs          = true
      metrics       = true
      ref_id        = "main-elasticsearch"
    }
  }
}

resource "ec_deployment" "deployment_monitor" {
  count = var.monitor_deployment ? 1 : 0

  name                   = "monitor-${local.name_prefix}-${local.version}"
  version                = data.ec_stack.deployment_version.version
  region                 = local.region
  deployment_template_id = local.deployment_template

  elasticsearch {
    topology {
      id   = "hot_content"
      size = "2g"
    }
  }
  kibana {}
}

resource "local_file" "enable_expvar" {
  content = templatefile("${path.module}/scripts/enable_expvar.tftpl", {
    kibana_url       = ec_deployment.deployment.kibana.0.https_endpoint,
    elastic_password = ec_deployment.deployment.elasticsearch_password,
  })
  filename = "${path.module}/scripts/enable_expvar.sh"
}

resource "null_resource" "enable_expvar" {
  triggers = {
    shell_hash = local_file.enable_expvar.id
  }
  provisioner "local-exec" {
    command     = "scripts/enable_expvar.sh"
    interpreter = ["/bin/bash", "-c"]
    working_dir = path.module
  }
}
