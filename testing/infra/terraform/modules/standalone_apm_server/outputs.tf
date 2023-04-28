output "apm_secret_token" {
  value       = random_password.apm_secret_token.result
  description = "The APM Server secret token"
  sensitive   = true
}

output "apm_server_url" {
  value       = "${aws_instance.apm.public_ip}:${local.apm_port}"
  description = "The APM Server URL"
}
