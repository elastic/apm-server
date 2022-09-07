output "kibana_url" {
  value       = module.ec_deployment.kibana_url
  description = "The secure Kibana URL"
}

output "rally_coordinator_ip" {
  value       = module.rally_workers.rally_coordinator_ip
  description = "Public IP address of rally coordinator node"
}

output "rally_ssh_private_key" {
  value       = module.rally_workers.rally_ssh_private_key
  description = "Private key to login to rally nodes"
  sensitive   = true
}

output "rally_summary_report" {
  value       = module.rally_workers.rally_summary
  description = "Rally report"
}
