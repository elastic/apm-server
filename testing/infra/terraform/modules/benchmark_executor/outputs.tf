output "public_ip" {
  value       = module.ec2_instance.public_ip
  description = "The public IP for the worker VM"
}
