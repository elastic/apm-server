output "moxy_url" {
  value       = "http://${aws_instance.moxy.public_ip}:${local.moxy_port}"
  description = "The Moxy server URL"
}

output "moxy_password" {
  value       = random_password.moxy_password.result
  description = "The Moxy password for communication"
  sensitive   = true
}
