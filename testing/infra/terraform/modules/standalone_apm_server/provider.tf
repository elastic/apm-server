provider "aws" {
  region = var.worker_region
  default_tags {
    tags = var.tags
  }
}
