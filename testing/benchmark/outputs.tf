output "public_ip" {
  value       = module.benchmark_worker.public_ip
  description = "The worker public IP"
}

output "elasticsearch_url" {
  value       = module.ec_deployment.elasticsearch_url
  description = "The secure Elasticsearch URL"
}

output "elasticsearch_username" {
  value       = module.ec_deployment.elasticsearch_username
  description = "The Elasticsearch username"
  sensitive   = true
}

output "elasticsearch_password" {
  value       = module.ec_deployment.elasticsearch_password
  description = "The Elasticsearch password"
  sensitive   = true
}

output "kibana_url" {
  value       = module.ec_deployment.kibana_url
  description = "The secure Kibana URL"
}
output "apm_secret_token" {
  value       = module.ec_deployment.apm_secret_token
  description = "The APM Server secret token"
  sensitive   = true
}

output "apm_server_url" {
  value       = module.ec_deployment.apm_url
  description = "The APM Server URL"
}

output "admin_console_url" {
  value       = module.ec_deployment.admin_console_url
  description = "The admin console URL"
}
