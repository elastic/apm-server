module "ec_deployment" {
  source = "../../testing/infra/terraform/modules/ec_deployment"
  region = var.ec_region

  deployment_template    = var.ec_deployment_template
  deployment_name_prefix = var.name

  // self monitoring is enabled so we can inspect Elasticsearch
  // logs from tests.
  observability_deployment = "self"

  apm_server_size = "1g"

  elasticsearch_size       = "4g"
  elasticsearch_zone_count = 1

  stack_version       = var.stack_version
  integrations_server = var.integrations_server

  docker_image              = var.ec_docker_image_override
  docker_image_tag_override = var.ec_docker_image_tag_override

  tags = merge(local.ci_tags, module.tags.tags)
}
