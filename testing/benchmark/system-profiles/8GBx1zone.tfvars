user_name = "USER"

# APM bench

stack_version = "8.16.0-SNAPSHOT"
ess_region = "gcp-us-west2"
deployment_template = "gcp-cpu-optimized"
worker_instance_type = "c6i.large"

# Elastic Cloud

apm_server_size = "8g"
apm_server_zone_count = 1

elasticsearch_size = "64g"
elasticsearch_zone_count = 2
elasticsearch_dedicated_masters = true

# Docker Images

docker_image_tag_override = {
  "elasticsearch" : "8.2.0-SNAPSHOT",
  "kibana" : "8.2.0-SNAPSHOT",
  "apm" : "8.16.0-SNAPSHOT-rubenvanstaden-1728505208",
}

docker_image_override = {
   "elasticsearch" : "docker.elastic.co/cloud-release/elasticsearch-cloud-ess",
   "kibana" : "docker.elastic.co/cloud-release/kibana-cloud",
   "apm" : "docker.elastic.co/observability-ci/elastic-agent-cloud",
}

# Standalone

standalone_apm_server_instance_size = "c6i.large"
standalone_moxy_instance_size       = "c6i.xlarge"
