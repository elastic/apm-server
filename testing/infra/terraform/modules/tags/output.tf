locals {
  tags = {
    "division" : "engineering"
    "org" : "obs"
    "team" : "apm-server"
    "project" : var.project
    "build" : var.build
<<<<<<< HEAD
    "ephemeral" : "true"
=======
>>>>>>> b4c79b3ab (terraform: enable build tag (#13841))
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
