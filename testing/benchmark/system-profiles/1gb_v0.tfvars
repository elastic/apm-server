user_name = "USER"

ess_region = "us-west-2"
deployment_template = "aws-cpu-optimized"
stack_version = "8.15.2"

# APM bench

worker_instance_type = "c6i.large"
worker_region = "us-west-2"

# Elastic Cloud

apm_server_size = "1g"
apm_server_zone_count = 1

elasticsearch_size = "16g"
elasticsearch_zone_count = 2

# Standalone

standalone_apm_server_instance_size = "c6i.large"
standalone_moxy_instance_size       = "c6i.xlarge"
