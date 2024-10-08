user_name = "USER"

# APM bench

stack_version = "8.16.0-SNAPSHOT"
ess_region = "gcp-us-west2"
deployment_template = "gcp-cpu-optimized"
worker_instance_type = "c6i.large"

# Elastic Cloud

apm_server_zone_count = 1
elasticsearch_size = "256g"
elasticsearch_zone_count = 2
apm_server_size = "1g"

# Standalone

standalone_apm_server_instance_size = "c6i.large"
standalone_moxy_instance_size       = "c6i.xlarge"
