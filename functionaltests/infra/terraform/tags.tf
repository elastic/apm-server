resource "time_static" "created_date" {}

locals {
  ci_tags = {
    environment  = var.ENVIRONMENT
    repo         = var.REPO
    branch       = var.BRANCH
    build        = var.BUILD_ID
    created_date = coalesce(var.CREATED_DATE, time_static.created_date.unix)
  }
  project = "apm-server-functionaltest"
}

module "tags" {
  source = "../../../../testing/infra/terraform/modules/tags"
  # use the convention for team/shared owned resources if we are running in CI. 
  # assume this is an individually owned resource otherwise. 
  project = local.project
}

