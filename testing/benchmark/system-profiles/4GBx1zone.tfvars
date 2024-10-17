user_name = "USER"

ess_region = "gcp-us-west2"
deployment_template = "gcp-cpu-optimized"
stack_version = "8.15.2"

# APM bench

worker_instance_type = "c6i.large"
worker_region = "us-west-2"

# Elastic Cloud

apm_server_size = "4g"
apm_server_zone_count = 1

elasticsearch_size = "32g"
elasticsearch_zone_count = 2

# Standalone

standalone_apm_server_instance_size = "c6i.large"
standalone_moxy_instance_size       = "c6i.xlarge"
