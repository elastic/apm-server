user_name = "USER"

# APM bench

worker_instance_type = "c6i.4xlarge"

# Elastic Cloud
ess_region = "gcp-us-west2"
deployment_template = "gcp-cpu-optimized"
# The number of AZs the APM Server should span.
apm_server_zone_count = 1
# The Elasticsearch cluster node size.
elasticsearch_size = "256g"
# The number of AZs the Elasticsearch cluster should have.
elasticsearch_zone_count = 2
# Run the cluster with a dedicated master
elasticsearch_dedicated_masters = true
# APM server instance size
apm_server_size = "32g"
# Number of shards for the ES indices
apm_shards = 4

# Standalone

standalone_apm_server_instance_size = "c6i.4xlarge"
standalone_moxy_instance_size       = "c6i.8xlarge"
