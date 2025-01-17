output "apm_url" {
  value = module.ec_deployment.apm_url
}

output "es_url" {
  value = module.ec_deployment.elasticsearch_url
}

output "username" {
  value = module.ec_deployment.elasticsearch_username
}
output "password" {
  value     = module.ec_deployment.elasticsearch_password
  sensitive = true
}

output "kb_url" {
  value = module.ec_deployment.kibana_url
}
