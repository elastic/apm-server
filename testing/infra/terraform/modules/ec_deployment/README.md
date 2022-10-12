<!-- BEGIN_TF_DOCS -->
# Elastic Cloud Deployment module

This module can be used to create an opinionated Elastic Cloud deployment with the configured inputs.
It requires access to either ESS or an ECE installation. To see which environment variables can be
used to configure the module, please refer to the [EC Provider docs](https://registry.terraform.io/providers/elastic/ec/latest/docs#authentication).

## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_ec"></a> [ec](#requirement\_ec) | >=0.4.1 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_ec"></a> [ec](#provider\_ec) | 0.4.0 |
| <a name="provider_external"></a> [external](#provider\_external) | n/a |
| <a name="provider_local"></a> [local](#provider\_local) | n/a |
| <a name="provider_null"></a> [null](#provider\_null) | n/a |

## Resources

| Name | Type |
|------|------|
| [ec_deployment.deployment](https://registry.terraform.io/providers/elastic/ec/latest/docs/resources/deployment) | resource |
| [ec_deployment.deployment_monitor](https://registry.terraform.io/providers/elastic/ec/latest/docs/resources/deployment) | resource |
| [local_file.custom_apm_integration_pkg](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.drop_pipeline](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.enable_expvar](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.secret_token](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.shard_settings](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [null_resource.custom_apm_integration_pkg](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.drop_pipeline](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.enable_expvar](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.secret_token](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.shard_settings](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [ec_stack.deployment_version](https://registry.terraform.io/providers/elastic/ec/latest/docs/data-sources/stack) | data source |
| [external_external.secret_token](https://registry.terraform.io/providers/hashicorp/external/latest/docs/data-sources/external) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apm_index_shards"></a> [apm\_index\_shards](#input\_apm\_index\_shards) | The number of shards to set for APM Indices | `number` | `0` | no |
| <a name="input_apm_server_expvar"></a> [apm\_server\_expvar](#input\_apm\_server\_expvar) | Whether or not to enable APM Server's expvar endpoint. Defaults to false | `bool` | `false` | no |
| <a name="input_apm_server_pprof"></a> [apm\_server\_pprof](#input\_apm\_server\_pprof) | Whether or not to enable APM Server's pprof endpoint. Defaults to false | `bool` | `false` | no |
| <a name="input_apm_server_size"></a> [apm\_server\_size](#input\_apm\_server\_size) | Optional apm server instance size | `string` | `"1g"` | no |
| <a name="input_apm_server_zone_count"></a> [apm\_server\_zone\_count](#input\_apm\_server\_zone\_count) | Optional apm server zone count | `number` | `1` | no |
| <a name="input_custom_apm_integration_pkg_path"></a> [custom\_apm\_integration\_pkg\_path](#input\_custom\_apm\_integration\_pkg\_path) | Path to the zipped custom APM integration package, if empty custom apm integration pkg is not installed | `string` | `""` | no |
| <a name="input_deployment_name_prefix"></a> [deployment\_name\_prefix](#input\_deployment\_name\_prefix) | Optional ESS or ECE region. Defaults to GCP US West 2 (Los Angeles) | `string` | `"apmserver"` | no |
| <a name="input_deployment_template"></a> [deployment\_template](#input\_deployment\_template) | Optional deployment template. Defaults to the CPU optimized template for GCP | `string` | `"gcp-compute-optimized-v2"` | no |
| <a name="input_docker_image"></a> [docker\_image](#input\_docker\_image) | Optional docker image overrides. The full map needs to be specified | `map(string)` | <pre>{<br>  "apm": "docker.elastic.co/cloud-release/elastic-agent-cloud",<br>  "elasticsearch": "docker.elastic.co/cloud-release/elasticsearch-cloud-ess",<br>  "kibana": "docker.elastic.co/cloud-release/kibana-cloud"<br>}</pre> | no |
| <a name="input_docker_image_tag_override"></a> [docker\_image\_tag\_override](#input\_docker\_image\_tag\_override) | Optional docker image tag overrides, The full map needs to be specified | `map(string)` | <pre>{<br>  "apm": "",<br>  "elasticsearch": "",<br>  "kibana": ""<br>}</pre> | no |
| <a name="input_drop_pipeline"></a> [drop\_pipeline](#input\_drop\_pipeline) | Whether or not to install an Elasticsearch ingest pipeline to drop all incoming APM documents. Defaults to false | `bool` | `false` | no |
| <a name="input_elasticsearch_autoscale"></a> [elasticsearch\_autoscale](#input\_elasticsearch\_autoscale) | Optional autoscale the Elasticsearch cluster | `bool` | `false` | no |
| <a name="input_elasticsearch_dedicated_masters"></a> [elasticsearch\_dedicated\_masters](#input\_elasticsearch\_dedicated\_masters) | Optionally use dedicated masters for the Elasticsearch cluster | `bool` | `false` | no |
| <a name="input_elasticsearch_size"></a> [elasticsearch\_size](#input\_elasticsearch\_size) | Optional Elasticsearch instance size | `string` | `"8g"` | no |
| <a name="input_elasticsearch_zone_count"></a> [elasticsearch\_zone\_count](#input\_elasticsearch\_zone\_count) | Optional Elasticsearch zone count | `number` | `2` | no |
| <a name="input_integrations_server"></a> [integrations\_server](#input\_integrations\_server) | Optionally disable the integrations server block and use the apm block (7.x only) | `bool` | `true` | no |
| <a name="input_monitor_deployment"></a> [monitor\_deployment](#input\_monitor\_deployment) | Optionally monitor the deployment in a separate deployment | `bool` | `false` | no |
| <a name="input_region"></a> [region](#input\_region) | Optional ESS or ECE region. Defaults to GCP US West 2 (Los Angeles) | `string` | `"gcp-us-west2"` | no |
| <a name="input_stack_version"></a> [stack\_version](#input\_stack\_version) | Optional stack version | `string` | `"latest"` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Optional set of tags to use for all deployments | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_admin_console_url"></a> [admin\_console\_url](#output\_admin\_console\_url) | n/a |
| <a name="output_apm_secret_token"></a> [apm\_secret\_token](#output\_apm\_secret\_token) | The APM Secret token |
| <a name="output_apm_url"></a> [apm\_url](#output\_apm\_url) | The secure APM URL |
| <a name="output_elasticsearch_password"></a> [elasticsearch\_password](#output\_elasticsearch\_password) | The Elasticsearch password |
| <a name="output_elasticsearch_url"></a> [elasticsearch\_url](#output\_elasticsearch\_url) | The secure Elasticsearch URL |
| <a name="output_elasticsearch_username"></a> [elasticsearch\_username](#output\_elasticsearch\_username) | The Elasticsearch username |
| <a name="output_kibana_url"></a> [kibana\_url](#output\_kibana\_url) | The secure Kibana URL |
| <a name="output_stack_version"></a> [stack\_version](#output\_stack\_version) | The matching stack pack version from the provided stack\_version |
<!-- END_TF_DOCS -->