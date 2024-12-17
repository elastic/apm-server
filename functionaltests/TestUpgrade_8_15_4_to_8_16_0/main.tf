resource "ec_deployment" "example_minimal" {
  name = var.name

  region                 = "aws-eu-west-1"
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
