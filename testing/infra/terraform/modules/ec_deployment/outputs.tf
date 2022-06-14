output "elasticsearch_url" {
  value       = ec_deployment.deployment.elasticsearch.0.https_endpoint
  description = "The secure Elasticsearch URL"
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
  value       = var.integrations_server ? jsonencode("secret.json") : ec_deployment.deployment.apm_secret_token
  sensitive   = true
  description = "The APM Secret token"
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
