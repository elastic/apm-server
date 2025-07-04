user_name = "USER"

# APM bench

worker_instance_type = "c6i.large"

# Elastic Cloud
ess_region          = "gcp-us-west2"
deployment_template = "gcp-cpu-optimized"
# The number of AZs the APM Server should span.
apm_server_zone_count = 1
# The Elasticsearch cluster node size.
elasticsearch_size = "32g"
# The number of AZs the Elasticsearch cluster should have.
elasticsearch_zone_count = 2
# APM server instance size
apm_server_size = "4g"

# Standalone

standalone_apm_server_instance_size = "c6i.large"
standalone_moxy_instance_size       = "c6i.xlarge"
