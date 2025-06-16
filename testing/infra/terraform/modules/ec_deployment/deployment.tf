locals {
  region              = var.region
  version             = var.stack_version
  deployment_template = var.deployment_template
  name_prefix         = var.deployment_name_prefix

  disable_observability   = var.observability_deployment == "none"
  self_observability      = var.observability_deployment == "self"
  dedicated_observability = var.observability_deployment == "dedicated"
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
    dynamic "topology" {
      for_each = var.elasticsearch_dedicated_masters ? [3] : []
      content {
        id         = "master"
        size       = "1g"
        zone_count = topology.value # Dedicated masters always need to be set in all zones
      }
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
    for_each = local.disable_observability ? [] : [1]
    content {
      deployment_id = var.observability_deployment
    }
  }
}

#ref_id        = "main-elasticsearch"

resource "ec_deployment" "dedicated_observability_deployment" {
  count = local.dedicated_observability ? 1 : 0

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

resource "local_file" "enable_features" {
  content = templatefile("${path.module}/scripts/enable_features.tftpl", {
    kibana_url                  = ec_deployment.deployment.kibana.0.https_endpoint,
    elastic_password            = ec_deployment.deployment.elasticsearch_password,
    enable_expvar               = var.apm_server_expvar
    enable_pprof                = var.apm_server_pprof
    enable_tail_sampling        = var.apm_server_tail_sampling
    tail_sampling_storage_limit = var.apm_server_tail_sampling_storage_limit
    tail_sampling_sample_rate   = var.apm_server_tail_sampling_sample_rate
  })
  filename = "${path.module}/scripts/enable_features.sh"
}

locals {
  secret_token_file = "${path.cwd}/secret_token_value.json"
}

resource "local_file" "secret_token" {
  count = var.integrations_server ? 1 : 0
  content = templatefile("${path.module}/scripts/secret_token.tftpl", {
    kibana_url        = ec_deployment.deployment.kibana.0.https_endpoint,
    elastic_password  = ec_deployment.deployment.elasticsearch_password,
    secret_token_file = local.secret_token_file
  })
  filename = "${path.module}/scripts/secret_token.sh"
}

resource "local_file" "shard_settings" {
  count = var.apm_index_shards > 0 ? 1 : 0
  content = templatefile("${path.module}/scripts/index_shards.tftpl", {
    elasticsearch_url      = ec_deployment.deployment.elasticsearch.0.https_endpoint,
    elasticsearch_password = ec_deployment.deployment.elasticsearch_password,
    elasticsearch_username = ec_deployment.deployment.elasticsearch_username,
    shards                 = var.apm_index_shards,
  })
  filename = "${path.module}/scripts/index_shards.sh"
}

resource "local_file" "custom_apm_integration_pkg" {
  count = var.custom_apm_integration_pkg_path != "" ? 1 : 0
  content = templatefile("${path.module}/scripts/custom-apm-integration-pkg.tftpl", {
    kibana_url                      = ec_deployment.deployment.kibana.0.https_endpoint,
    elastic_password                = ec_deployment.deployment.elasticsearch_password,
    custom_apm_integration_pkg_path = var.custom_apm_integration_pkg_path,
  })
  filename = "${path.module}/scripts/custom-apm-integration-pkg.sh"
}

resource "null_resource" "enable_features" {
  triggers = {
    shell_hash          = local_file.enable_features.id
    integrations_server = var.integrations_server
  }
  provisioner "local-exec" {
    command     = "scripts/enable_features.sh"
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
# to extract it from there.
# Load it from secret_token_file as a sensitive variable.
data "local_sensitive_file" "secret_token" {
  count = var.integrations_server ? 1 : 0

  filename   = local.secret_token_file
  depends_on = [null_resource.secret_token]
}

resource "null_resource" "shard_settings" {
  count = var.apm_index_shards > 0 ? 1 : 0
  triggers = {
    deployment_id = ec_deployment.deployment.id
    shell_hash    = local_file.shard_settings.0.id
  }
  provisioner "local-exec" {
    command     = "scripts/index_shards.sh"
    interpreter = ["/bin/bash", "-c"]
    working_dir = path.module
  }
}

resource "null_resource" "custom_apm_integration_pkg" {
  count = var.custom_apm_integration_pkg_path != "" ? 1 : 0
  triggers = {
    deployment_id = ec_deployment.deployment.id
    pkg_update    = filesha256(var.custom_apm_integration_pkg_path)
  }
  provisioner "local-exec" {
    command     = "scripts/custom-apm-integration-pkg.sh"
    interpreter = ["/bin/bash", "-c"]
    working_dir = path.module
  }
}

resource "null_resource" "drop_pipeline" {
  count = var.drop_pipeline ? 1 : 0
  triggers = {
    deployment_id = ec_deployment.deployment.id
    shell_hash    = local_file.drop_pipeline.0.id
  }
  provisioner "local-exec" {
    command     = "scripts/drop_pipeline.sh"
    interpreter = ["/bin/bash", "-c"]
    working_dir = path.module
  }
}

resource "local_file" "drop_pipeline" {
  count = var.drop_pipeline ? 1 : 0
  content = templatefile("${path.module}/scripts/drop_pipeline.tftpl", {
    elasticsearch_url      = ec_deployment.deployment.elasticsearch.0.https_endpoint,
    elasticsearch_password = ec_deployment.deployment.elasticsearch_password,
    elasticsearch_username = ec_deployment.deployment.elasticsearch_username,
  })
  filename = "${path.module}/scripts/drop_pipeline.sh"
}
