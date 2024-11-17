user_name = "USER"

ess_region = "us-west-2"
deployment_template = "gcp-cpu-optimized"
stack_version = "8.16.0"

# APM bench

worker_instance_type = "c6i.xlarge"
worker_region = "us-west-2"

# Elastic Cloud

apm_server_size = "8g"
apm_server_zone_count = 1

elasticsearch_size = "64g"
elasticsearch_zone_count = 2

# Standalone

standalone_apm_server_instance_size = "c6i.xlarge"
standalone_moxy_instance_size       = "c6i.2xlarge"
