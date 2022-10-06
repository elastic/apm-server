output "rally_summary" {
  value       = data.remote_file.rally_summary.content
  description = "Summary of rally run"
}

output "rally_coordinator_ip" {
  value       = google_compute_instance.rally_nodes[0].network_interface[0].access_config[0].nat_ip
  description = "Public IP address of rally coordinator node"
}

output "rally_ssh_private_key" {
  value       = tls_private_key.rally.private_key_openssh
  description = "Private key to login to rally nodes"
  sensitive   = true
}
