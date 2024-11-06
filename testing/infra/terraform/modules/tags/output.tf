locals {
  tags = {
    "division" : "engineering"
    "org" : "obs"
    "team" : "obs-ds-intake-services"
    "project" : var.project
    "build" : var.build
    "ephemeral" : "true"
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
