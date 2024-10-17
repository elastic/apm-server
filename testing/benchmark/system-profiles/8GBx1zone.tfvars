user_name = "USER"

ess_region = "gcp-us-west2"
deployment_template = "gcp-cpu-optimized"
stack_version = "8.16.0-SNAPSHOT"

# APM bench

worker_instance_type = "c6i.xlarge"
worker_region = "us-west-2"

# Elastic Cloud

# The number of AZs the APM Server should span.
apm_server_zone_count = 1
# The Elasticsearch cluster node size.
elasticsearch_size = "64g"
# The number of AZs the Elasticsearch cluster should have.
elasticsearch_zone_count = 2
# APM server instance size
apm_server_size = "8g"

# Standalone

standalone_apm_server_instance_size = "c6i.xlarge"
standalone_moxy_instance_size       = "c6i.2xlarge"
