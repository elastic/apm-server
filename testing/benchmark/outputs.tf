locals {
  ec_target = lower(var.ec_target)
  adminconsole_urls = {
    qa   = "https://admin.qa.cld.elstc.co"
    pro  = "https://cloud.elastic.co"
  }
  ec_console_url = local.adminconsole_urls[local.ec_target]
}

output "public_ip" {
  value       = module.benchmark_worker.public_ip
  description = "The worker public IP"
}

output "elasticsearch_url" {
  value       = var.run_standalone ? module.moxy[0].moxy_url : module.ec_deployment[0].elasticsearch_url
  description = "The secure Elasticsearch URL"
}

output "elasticsearch_username" {
  value       = var.run_standalone ? "elastic" : module.ec_deployment[0].elasticsearch_username
  description = "The Elasticsearch username"
  sensitive   = true
}

output "elasticsearch_password" {
  value       = var.run_standalone ? module.moxy[0].moxy_password : module.ec_deployment[0].elasticsearch_password
  description = "The Elasticsearch password"
  sensitive   = true
}

output "kibana_url" {
  value       = var.run_standalone ? "" : module.ec_deployment[0].kibana_url
  description = "The secure Kibana URL"
}

output "apm_secret_token" {
  value       = var.run_standalone ? module.standalone_apm_server[0].apm_secret_token : module.ec_deployment[0].apm_secret_token
  description = "The APM Server secret token"
  sensitive   = true
}

output "apm_server_url" {
  value       = var.run_standalone ? module.standalone_apm_server[0].apm_server_url : module.ec_deployment[0].apm_url
  description = "The APM Server URL"
  sensitive   = true
}

output "apm_server_ip" {
  value       = var.run_standalone ? module.standalone_apm_server[0].apm_server_ip : ""
  description = "The APM Server EC2 IP address"
}

output "moxy_ip" {
  value       = var.run_standalone ? module.moxy[0].moxy_ip : ""
  description = "The Moxy EC2 IP address"
}

output "admin_console_url" {
  value       = var.run_standalone ? "${local.ec_console_url}/deployments" : "${local.ec_console_url}/deployments/${module.ec_deployment[0].deployment_id}/integrations_server"
  description = "The admin console URL"
}
