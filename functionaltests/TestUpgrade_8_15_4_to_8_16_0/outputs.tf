output "deployment_id" {
  value = ec_deployment.example_minimal.id
}

output "apm_url" {
  value = ec_deployment.example_minimal.integrations_server.endpoints.apm
}

output "es_url" {
  value = ec_deployment.example_minimal.elasticsearch.https_endpoint
}

output "username" {
  value = ec_deployment.example_minimal.elasticsearch_username
}
output "password" {
  value     = ec_deployment.example_minimal.elasticsearch_password
  sensitive = true
}

output "kb_url" {
  value = ec_deployment.example_minimal.kibana.https_endpoint
}
