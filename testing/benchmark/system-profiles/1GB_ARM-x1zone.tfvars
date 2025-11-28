user_name = "USER"

# APM bench

worker_instance_type = "c6g.large"

# Elastic Cloud

# The number of AZs the APM Server should span.
apm_server_zone_count = 1
# The Elasticsearch cluster node size.
elasticsearch_size = "16g"
# The number of AZs the Elasticsearch cluster should have.
elasticsearch_zone_count = 2
# APM server instance size
apm_server_size = "1g"

# Standalone

standalone_apm_server_instance_size = "c6g.large"
standalone_moxy_instance_size       = "c6g.xlarge"
