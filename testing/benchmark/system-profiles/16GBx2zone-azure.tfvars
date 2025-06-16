user_name = "USER"

# APM bench

worker_instance_type = "c6i.2xlarge"

# Elastic Cloud
ess_region          = "azure-westus2"
deployment_template = "azure-cpu-optimized"
# The number of AZs the APM Server should span.
apm_server_zone_count = 1
# The Elasticsearch cluster node size.
elasticsearch_size = "120g"
# The number of AZs the Elasticsearch cluster should have.
elasticsearch_zone_count = 2
# APM server instance size
apm_server_size = "15g"
# Number of shards for the ES indices
apm_shards = 4

# Standalone

standalone_apm_server_instance_size = "c6i.2xlarge"
standalone_moxy_instance_size       = "c6i.4xlarge"
