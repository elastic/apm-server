locals {
  tags = {
    "division" : "engineering"
    "org" : "obs"
    "team" : "apm-server"
    "project" : var.project
    "build" : var.build
  }
}

output "tags" {
  value       = local.tags
  description = "Tags for CSP resources"
}

output "labels" {
  value       = local.tags
  description = "Labels for CSP resources"
}
