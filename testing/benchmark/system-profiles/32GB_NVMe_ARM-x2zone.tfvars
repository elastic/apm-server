user_name = "USER"

# APM bench

worker_instance_type = "c6g.4xlarge"

# Elastic Cloud

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

standalone_apm_server_instance_size = "c6gd.4xlarge"
standalone_moxy_instance_size       = "c6g.8xlarge"
