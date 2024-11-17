user_name = "USER"

ess_region = "azure-westus2"
deployment_template = "azure-cpu-optimized"
stack_version = "8.16.0"

# APM bench

worker_instance_type = "c6i.2xlarge"
worker_region = "us-west-2"

# Elastic Cloud

apm_server_size = "32g"
apm_server_zone_count = 1
apm_shards = 4

elasticsearch_size = "256g"
elasticsearch_zone_count = 2
elasticsearch_dedicated_masters = true

# Standalone

standalone_apm_server_instance_size = "c6i.4xlarge"
standalone_moxy_instance_size       = "c6i.8xlarge"
