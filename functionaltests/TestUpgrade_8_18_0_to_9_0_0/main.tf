module "ec_deployment" {
  source = "../../testing/infra/terraform/modules/ec_deployment"
  region = var.ec_region

  deployment_template    = "aws-storage-optimized"
  deployment_name_prefix = var.name

  // self monitoring is enabled so we can inspect Elasticsearch
  // logs from tests.
  observability_deployment = "self"

  apm_server_size = "1g"

  elasticsearch_size       = "4g"
  elasticsearch_zone_count = 1

  stack_version = var.stack_version

  tags = merge(local.ci_tags, module.tags.tags)
}
