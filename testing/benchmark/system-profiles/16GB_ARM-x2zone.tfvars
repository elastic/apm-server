user_name = "USER"

# APM bench

worker_instance_type = "c6g.2xlarge"

# Elastic Cloud
ess_region          = "us-west-2"
deployment_template = "aws-cpu-optimized-faster-warm"
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

# Standalone

standalone_apm_server_instance_size = "c6g.2xlarge"
standalone_moxy_instance_size       = "c6g.4xlarge"
