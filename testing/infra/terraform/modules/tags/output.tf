locals {
  tags = {
    "division" : "engineering"
    "org" : "obs"
    "team" : "apm-server"
    "project" : var.project
<<<<<<< HEAD
=======
    "build" : var.build
    "ephemeral" : "true"
>>>>>>> 812d7dde3 (ci: add ephemeral tag (#14179))
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
