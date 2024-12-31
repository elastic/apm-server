resource "ec_deployment" "example_minimal" {
  name = var.name

  region                 = var.ec_region
  version                = var.stack_version
  deployment_template_id = "aws-storage-optimized"

  elasticsearch = {
    hot = {
      autoscaling = {}
    }

    ml = {
      autoscaling = {
        autoscale = true
      }
    }
  }

  kibana = {
    topology = {}
  }

  integrations_server = {}

  tags = merge(local.ci_tags, module.tags.tags)
}
