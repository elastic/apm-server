output "public_ip" {
  value       = module.benchmark_worker.public_ip
  description = "The worker public IP"
}
