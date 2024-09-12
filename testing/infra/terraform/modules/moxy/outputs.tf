output "moxy_url" {
  value       = "http://${aws_instance.moxy.public_ip}:${local.moxy_port}"
  description = "The Moxy Server URL"
}
