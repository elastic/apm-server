<!-- BEGIN_TF_DOCS -->
# Benchmark executor module

This module creates a benchmark executor virtual machine in AWS on the specified region
so that `apmbench` can be run against a configured APM Server.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | n/a |
| <a name="provider_local"></a> [local](#provider\_local) | n/a |
| <a name="provider_null"></a> [null](#provider\_null) | n/a |

## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_ec2_instance"></a> [ec2\_instance](#module\_ec2\_instance) | terraform-aws-modules/ec2-instance/aws | 3.5.0 |
| <a name="module_vpc"></a> [vpc](#module\_vpc) | terraform-aws-modules/vpc/aws | 3.14.0 |

## Resources

| Name | Type |
|------|------|
| [aws_key_pair.worker](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/key_pair) | resource |
| [local_file.envrc](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [null_resource.apmbench](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.envrc](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [aws_ami.worker_ami](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/ami) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apm_secret_token"></a> [apm\_secret\_token](#input\_apm\_secret\_token) | n/a | `string` | `""` | no |
| <a name="input_apm_server_url"></a> [apm\_server\_url](#input\_apm\_server\_url) | Required APM Server URL | `string` | n/a | yes |
| <a name="input_apmbench_bin_path"></a> [apmbench\_bin\_path](#input\_apmbench\_bin\_path) | Optionally upload the apmbench binary from the specified path to the worker machine | `string` | `""` | no |
| <a name="input_instance_type"></a> [instance\_type](#input\_instance\_type) | Optional instance type to use for the worker VM | `string` | `"c6i.large"` | no |
| <a name="input_private_key"></a> [private\_key](#input\_private\_key) | n/a | `string` | `"~/.ssh/id_rsa_terraform"` | no |
| <a name="input_public_cidr"></a> [public\_cidr](#input\_public\_cidr) | n/a | `list(string)` | <pre>[<br>  "192.168.44.0/26",<br>  "192.168.44.64/26",<br>  "192.168.44.128/26"<br>]</pre> | no |
| <a name="input_public_key"></a> [public\_key](#input\_public\_key) | n/a | `string` | `"~/.ssh/id_rsa_terraform.pub"` | no |
| <a name="input_region"></a> [region](#input\_region) | n/a | `string` | `"us-west2"` | no |
| <a name="input_tags"></a> [tags](#input\_tags) | Optional set of tags to use for all resources | `map(string)` | `{}` | no |
| <a name="input_user_name"></a> [user\_name](#input\_user\_name) | Required username to use for resource name prefixes | `string` | n/a | yes |
| <a name="input_vpc_cidr"></a> [vpc\_cidr](#input\_vpc\_cidr) | n/a | `string` | `"192.168.44.0/24"` | no |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_public_ip"></a> [public\_ip](#output\_public\_ip) | The public IP for the worker VM |
<!-- END_TF_DOCS -->