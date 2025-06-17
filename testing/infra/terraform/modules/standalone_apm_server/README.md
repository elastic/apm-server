<!-- BEGIN_TF_DOCS -->
# Standalone APM Server module

This module can be used to create a standalone APM Server deployment against a set of predefined architectures and instance types.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | n/a |
| <a name="provider_external"></a> [external](#provider\_external) | n/a |
| <a name="provider_null"></a> [null](#provider\_null) | n/a |
| <a name="provider_random"></a> [random](#provider\_random) | n/a |

## Resources

| Name | Type |
|------|------|
| [aws_instance.apm](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/instance) | resource |
| [aws_key_pair.provisioner_key](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/key_pair) | resource |
| [aws_security_group.main](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [null_resource.apm_server_log](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [random_password.apm_secret_token](https://registry.terraform.io/providers/hashicorp/random/latest/docs/resources/password) | resource |
| [aws_ami.os](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/ami) | data source |
| [aws_region.current](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/region) | data source |
| [aws_subnets.public_subnets](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/subnets) | data source |
| [external_external.latest_apm_server](https://registry.terraform.io/providers/hashicorp/external/latest/docs/data-sources/external) | data source |
| [external_external.latest_elastic_agent](https://registry.terraform.io/providers/hashicorp/external/latest/docs/data-sources/external) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apm_instance_type"></a> [apm\_instance\_type](#input\_apm\_instance\_type) | Optional apm server instance type override | `string` | `""` | no |
| <a name="input_apm_iops"></a> [apm\_iops](#input\_apm\_iops) | Optional apm server disk IOPS override | `number` | `null` | no |
| <a name="input_apm_server_bin_path"></a> [apm\_server\_bin\_path](#input\_apm\_server\_bin\_path) | Optionally use the apm-server binary from the specified path instead | `string` | `""` | no |
| <a name="input_apm_server_tail_sampling"></a> [apm\_server\_tail\_sampling](#input\_apm\_server\_tail\_sampling) | Whether or not to enable APM Server tail-based sampling. Defaults to false | `bool` | `false` | no |
| <a name="input_apm_server_tail_sampling_sample_rate"></a> [apm\_server\_tail\_sampling\_sample\_rate](#input\_apm\_server\_tail\_sampling\_sample\_rate) | Sample rate of APM Server tail-based sampling. Defaults to 0.1 | `number` | `0.1` | no |
| <a name="input_apm_server_tail_sampling_storage_limit"></a> [apm\_server\_tail\_sampling\_storage\_limit](#input\_apm\_server\_tail\_sampling\_storage\_limit) | Storage size limit of APM Server tail-based sampling. Defaults to 10GB | `string` | `"10GB"` | no |
| <a name="input_apm_volume_size"></a> [apm\_volume\_size](#input\_apm\_volume\_size) | Optional apm server volume size in GB override | `number` | `null` | no |
| <a name="input_apm_volume_type"></a> [apm\_volume\_type](#input\_apm\_volume\_type) | Optional apm server volume type override | `string` | `null` | no |
| <a name="input_aws_os"></a> [aws\_os](#input\_aws\_os) | Optional aws EC2 instance OS | `string` | `""` | no |
| <a name="input_aws_provisioner_key_name"></a> [aws\_provisioner\_key\_name](#input\_aws\_provisioner\_key\_name) | ssh key name to create the aws key pair and remote provision the EC2 instance | `string` | n/a | yes |
| <a name="input_ea_managed"></a> [ea\_managed](#input\_ea\_managed) | Whether or not install Elastic Agent managed APM Server | `bool` | `false` | no |
| <a name="input_elasticsearch_password"></a> [elasticsearch\_password](#input\_elasticsearch\_password) | The Elasticsearch password | `string` | n/a | yes |
| <a name="input_elasticsearch_url"></a> [elasticsearch\_url](#input\_elasticsearch\_url) | The secure Elasticsearch URL | `string` | n/a | yes |
| <a name="input_elasticsearch_username"></a> [elasticsearch\_username](#input\_elasticsearch\_username) | The Elasticsearch username | `string` | n/a | yes |
| <a name="input_stack_version"></a> [stack\_version](#input\_stack\_version) | Optional stack version | `string` | `"latest"` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Optional set of tags to use for all deployments | `map(string)` | `{}` | no |
| <a name="input_vpc_id"></a> [vpc\_id](#input\_vpc\_id) | VPC ID to provision the EC2 instance | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_apm_secret_token"></a> [apm\_secret\_token](#output\_apm\_secret\_token) | The APM Server secret token |
| <a name="output_apm_server_ip"></a> [apm\_server\_ip](#output\_apm\_server\_ip) | The APM Server EC2 IP address |
| <a name="output_apm_server_url"></a> [apm\_server\_url](#output\_apm\_server\_url) | The APM Server URL |
<!-- END_TF_DOCS -->