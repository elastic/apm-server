<!-- BEGIN_TF_DOCS -->
# Elastic Cloud Deployment module

This module can be used to create an opinionated Elastic Cloud deployment with the configured inputs.
It requires access to either ESS or an ECE installation. To see which environment variables can be
used to configure the module, please refer to the [EC Provider docs](https://registry.terraform.io/providers/elastic/ec/latest/docs#authentication).

## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_ec"></a> [ec](#requirement\_ec) | >=0.4.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_ec"></a> [ec](#provider\_ec) | 0.4.0 |
| <a name="provider_local"></a> [local](#provider\_local) | n/a |
| <a name="provider_null"></a> [null](#provider\_null) | n/a |

## Resources

| Name | Type |
|------|------|
| [ec_deployment.deployment](https://registry.terraform.io/providers/elastic/ec/latest/docs/resources/deployment) | resource |
| [ec_deployment.deployment_monitor](https://registry.terraform.io/providers/elastic/ec/latest/docs/resources/deployment) | resource |
| [local_file.enable_expvar](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [null_resource.enable_expvar](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [ec_stack.deployment_version](https://registry.terraform.io/providers/elastic/ec/latest/docs/data-sources/stack) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apm_server_expvar"></a> [apm\_server\_expvar](#input\_apm\_server\_expvar) | Wether or not to enable APM Server's expvar endpoint. Defaults to true | `bool` | `true` | no |
| <a name="input_apm_server_pprof"></a> [apm\_server\_pprof](#input\_apm\_server\_pprof) | Wether or not to enable APM Server's pprof endpoint. Defaults to true | `bool` | `true` | no |
| <a name="input_apm_server_size"></a> [apm\_server\_size](#input\_apm\_server\_size) | Optional apm server instance size | `string` | `"1g"` | no |
| <a name="input_apm_server_zone_count"></a> [apm\_server\_zone\_count](#input\_apm\_server\_zone\_count) | Optional apm server zone count | `number` | `1` | no |
| <a name="input_deployment_name_prefix"></a> [deployment\_name\_prefix](#input\_deployment\_name\_prefix) | Optional ESS or ECE region. Defaults to GCP US West 2 (Los Angeles) | `string` | `"apmserver"` | no |
| <a name="input_deployment_template"></a> [deployment\_template](#input\_deployment\_template) | Optional deployment template. Defaults to the CPU optimized template for GCP | `string` | `"gcp-compute-optimized-v2"` | no |
| <a name="input_docker_image"></a> [docker\_image](#input\_docker\_image) | Optional docker image overrides. All | `map(string)` | <pre>{<br>  "apm": "docker.elastic.co/cloud-release/elastic-agent-cloud",<br>  "elasticsearch": "docker.elastic.co/cloud-release/elasticsearch-cloud-ess",<br>  "kibana": "docker.elastic.co/cloud-release/kibana-cloud"<br>}</pre> | no |
| <a name="input_docker_image_tag_override"></a> [docker\_image\_tag\_override](#input\_docker\_image\_tag\_override) | Optional docker image tag override | `string` | `""` | no |
| <a name="input_elasticsearch_size"></a> [elasticsearch\_size](#input\_elasticsearch\_size) | Optional Elasticsearch instance size | `string` | `"8g"` | no |
| <a name="input_elasticsearch_zone_count"></a> [elasticsearch\_zone\_count](#input\_elasticsearch\_zone\_count) | Optional Elasticsearch zone count | `number` | `2` | no |
| <a name="input_monitor_deployment"></a> [monitor\_deployment](#input\_monitor\_deployment) | Optionally monitor the deployment in a separate deployment | `bool` | `false` | no |
| <a name="input_region"></a> [region](#input\_region) | Optional ESS or ECE region. Defaults to GCP US West 2 (Los Angeles) | `string` | `"gcp-us-west2"` | no |
| <a name="input_stack_version"></a> [stack\_version](#input\_stack\_version) | Optional stack version | `string` | `"latest"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_apm_secret_token"></a> [apm\_secret\_token](#output\_apm\_secret\_token) | The APM Secret token |
| <a name="output_apm_url"></a> [apm\_url](#output\_apm\_url) | The secure APM URL |
| <a name="output_elasticsearch_password"></a> [elasticsearch\_password](#output\_elasticsearch\_password) | The Elasticsearch password |
| <a name="output_elasticsearch_url"></a> [elasticsearch\_url](#output\_elasticsearch\_url) | The secure Elasticsearch URL |
| <a name="output_elasticsearch_username"></a> [elasticsearch\_username](#output\_elasticsearch\_username) | The Elasticsearch username |
| <a name="output_kibana_url"></a> [kibana\_url](#output\_kibana\_url) | The secure Kibana URL |
<!-- END_TF_DOCS -->