user_name = "USER"

ess_region = "azure-westus2"
deployment_template = "azure-cpu-optimized"
stack_version = "8.15.2"

# APM bench

worker_instance_type = "c6i.2xlarge"
worker_region = "us-west-2"

# Elastic Cloud

apm_server_size = "15g"
apm_server_zone_count = 1
apm_shards = 4

elasticsearch_size = "120g"
elasticsearch_zone_count = 2

# Standalone

standalone_apm_server_instance_size = "c6i.2xlarge"
standalone_moxy_instance_size       = "c6i.4xlarge"
