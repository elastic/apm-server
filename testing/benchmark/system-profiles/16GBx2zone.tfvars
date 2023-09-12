user_name = "USER"

# The number of AZs the APM Server should span.
apm_server_zone_count = 1
# The Elasticsearch cluster node size.
elasticsearch_size = "128g"
# The number of AZs the Elasticsearch cluster should have.
elasticsearch_zone_count = 2
# APM server instance size
apm_server_size = "16g"
# Number of shards for the ES indices
apm_shards = 4
# Benchmarks executor size executor
worker_instance_type = "c6i.2xlarge"
