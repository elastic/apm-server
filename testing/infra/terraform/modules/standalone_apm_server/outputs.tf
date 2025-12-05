output "apm_secret_token" {
  value       = random_password.apm_secret_token.result
  description = "The APM Server secret token"
  sensitive   = true
}

output "apm_server_url" {
  value       = "http://${aws_instance.apm.public_ip}:${local.apm_port}"
  description = "The APM Server URL"
}

output "apm_server_ip" {
  value       = aws_instance.apm.public_ip
  description = "The APM Server EC2 IP address"
}

output "apm_server_ssh_user" {
  value       = local.image_ssh_users[var.aws_os]
  description = "The SSH user for the APM Server EC2 instance"
}

output "apm_server_os" {
  value       = var.aws_os
  description = "The operating system name for the APM Server EC2 instance"
}
