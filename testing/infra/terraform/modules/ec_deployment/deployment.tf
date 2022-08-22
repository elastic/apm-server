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
  name                   = "${local.name_prefix}-${data.ec_stack.deployment_version.version}"
  version                = data.ec_stack.deployment_version.version
  region                 = local.region
  deployment_template_id = local.deployment_template
  tags                   = var.tags

  elasticsearch {
    autoscale = var.elasticsearch_autoscale
    topology {
      id         = "hot_content"
      size       = var.elasticsearch_size
      zone_count = var.elasticsearch_zone_count
    }
    dynamic "config" {
      for_each = var.docker_image_tag_override["elasticsearch"] != "" ? [var.docker_image["elasticsearch"]] : []
      content {
        docker_image = "${config.value}:${var.docker_image_tag_override["elasticsearch"]}"
      }
    }
  }
  kibana {
    dynamic "config" {
      for_each = var.docker_image_tag_override["kibana"] != "" ? [var.docker_image["kibana"]] : []
      content {
        docker_image = "${config.value}:${var.docker_image_tag_override["kibana"]}"
      }
    }
  }
  dynamic "apm" {
    for_each = var.integrations_server ? [] : [1]
    content {
      dynamic "config" {
        for_each = var.docker_image_tag_override["apm"] != "" ? [var.docker_image["apm"]] : []
        content {
          docker_image = "${config.value}:${var.docker_image_tag_override["apm"]}"
        }
      }
      topology {
        size       = var.apm_server_size
        zone_count = var.apm_server_zone_count
      }
    }
  }

  dynamic "integrations_server" {
    for_each = var.integrations_server ? [1] : []
    content {
      dynamic "config" {
        for_each = var.docker_image_tag_override["apm"] != "" ? [var.docker_image["apm"]] : []
        content {
          docker_image = "${config.value}:${var.docker_image_tag_override["apm"]}"
        }
      }
      topology {
        size       = var.apm_server_size
        zone_count = var.apm_server_zone_count
      }
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
  tags                   = var.tags

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
    enable_expvar    = var.apm_server_expvar
    enable_pprof     = var.apm_server_pprof
  })
  filename = "${path.module}/scripts/enable_expvar.sh"
}

resource "local_file" "secret_token" {
  count = var.integrations_server ? 1 : 0
  content = templatefile("${path.module}/scripts/secret_token.tftpl", {
    kibana_url       = ec_deployment.deployment.kibana.0.https_endpoint,
    elastic_password = ec_deployment.deployment.elasticsearch_password,
  })
  filename = "${path.module}/scripts/secret_token.sh"
}

resource "null_resource" "enable_expvar" {
  triggers = {
    shell_hash          = local_file.enable_expvar.id
    integrations_server = var.integrations_server
  }
  provisioner "local-exec" {
    command     = "scripts/enable_expvar.sh"
    interpreter = ["/bin/bash", "-c"]
    working_dir = path.module
  }
}

resource "null_resource" "secret_token" {
  count = var.integrations_server ? 1 : 0
  triggers = {
    deployment_id = ec_deployment.deployment.id
    shell_hash    = local_file.secret_token.0.id
  }
  provisioner "local-exec" {
    command     = "scripts/secret_token.sh"
    interpreter = ["/bin/bash", "-c"]
    working_dir = path.module
  }
}

# Since the secret token value is set in the APM Integration policy, we need
# an "external" resource to run a shell script that returns the secret token
# as {"value":"SECRET_TOKEN"}.
data "external" "secret_token" {
  count       = var.integrations_server ? 1 : 0
  depends_on  = [local_file.secret_token]
  program     = ["/bin/bash", "-c", "scripts/secret_token.sh"]
  working_dir = path.module
}
