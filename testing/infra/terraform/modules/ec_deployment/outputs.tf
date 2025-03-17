output "deployment_id" {
  value       = ec_deployment.deployment.id
  description = "The deployment ID for the created cluster"
}

output "kibana_url" {
  value       = ec_deployment.deployment.kibana.0.https_endpoint
  description = "The secure Kibana URL"
}

output "apm_url" {
  value       = var.integrations_server ? ec_deployment.deployment.integrations_server.0.https_endpoint : ec_deployment.deployment.apm.0.https_endpoint
  description = "The secure APM URL"
}

output "apm_secret_token" {
  value       = var.integrations_server ? chomp(data.local_sensitive_file.secret_token[0].content) : ec_deployment.deployment.apm_secret_token
  sensitive   = true
  description = "The APM Secret token"
}

output "elasticsearch_url" {
  value       = ec_deployment.deployment.elasticsearch.0.https_endpoint
  description = "The secure Elasticsearch URL"
  depends_on = [
    null_resource.shard_settings,
    null_resource.custom_apm_integration_pkg,
  ]
}

output "elasticsearch_username" {
  value       = ec_deployment.deployment.elasticsearch_username
  sensitive   = true
  description = "The Elasticsearch username"
}

output "elasticsearch_password" {
  value       = ec_deployment.deployment.elasticsearch_password
  sensitive   = true
  description = "The Elasticsearch password"
}

output "stack_version" {
  value       = data.ec_stack.deployment_version.version
  description = "The matching stack pack version from the provided stack_version"
}

output "admin_console_url" {
  value = "https://admin.found.no/deployments/${ec_deployment.deployment.id}/integrations_server"
}

output "ech_apm_component_id" {
  value       = var.integrations_server ? ec_deployment.deployment.integrations_server[0].resource_id : ec_deployment.deployment.apm[0].resource_id
  description = "APM or Integrations Server Cloud component ID"
}
