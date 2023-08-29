user_name = "USER"

# The number of AZs the APM Server should span.
apm_server_zone_count = 1
# The Elasticsearch cluster node size.
elasticsearch_size = "245760"
elasticsearch_dedicated_masters = true
# The number of AZs the Elasticsearch cluster should have.
elasticsearch_zone_count = 1
# APM server instance size
apm_server_size = "32g"
# Benchmarks executor size executor
worker_instance_type = "c6i.2xlarge"

ess_region = "us-east-1"
deployment_template = "aws-cpu-optimized-v5"
