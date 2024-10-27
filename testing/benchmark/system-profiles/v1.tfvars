user_name = "USER"

ess_region = "gcp-us-west2"
deployment_template = "gcp-cpu-optimized"
stack_version = "8.16.0-SNAPSHOT"

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

docker_image_override = {
    "elasticsearch":"docker.elastic.co/cloud-release/elasticsearch-cloud-ess",
    "kibana":"docker.elastic.co/cloud-release/kibana-cloud",
    "apm":"docker.elastic.co/observability-ci/elastic-agent",
}

docker_image_tag_override = {
    "elasticsearch":"8.16.0-SNAPSHOT",
    "kibana":"8.16.0-SNAPSHOT",
    "apm":"8.16.0-SNAPSHOT-rubenvanstaden-1729473426",
}

# Standalone

standalone_apm_server_instance_size = "c6i.4xlarge"
standalone_moxy_instance_size       = "c6i.8xlarge"
