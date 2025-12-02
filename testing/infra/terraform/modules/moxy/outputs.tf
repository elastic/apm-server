output "moxy_url" {
  value       = "http://${aws_instance.moxy.public_ip}:${local.moxy_port}"
  description = "Moxy server URL"
}

output "moxy_ip" {
  value       = aws_instance.moxy.public_ip
  description = "Moxy server IP address"
}

output "moxy_password" {
  value       = random_password.moxy_password.result
  description = "Moxy server password"
  sensitive   = true
}
