<!-- BEGIN_TF_DOCS -->
# Moxy module

This module can be used to create a stub ElasticSearch deployment (Moxy).

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | n/a |
| <a name="provider_random"></a> [random](#provider\_random) | n/a |

## Resources

| Name | Type |
|------|------|
| [aws_instance.moxy](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/instance) | resource |
| [aws_key_pair.provisioner_key](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/key_pair) | resource |
| [aws_security_group.main](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/security_group) | resource |
| [random_password.moxy_password](https://registry.terraform.io/providers/hashicorp/random/latest/docs/resources/password) | resource |
| [aws_ami.worker_ami](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/ami) | data source |
| [aws_subnets.public_subnets](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/data-sources/subnets) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_aws_provisioner_key_name"></a> [aws\_provisioner\_key\_name](#input\_aws\_provisioner\_key\_name) | ssh key name to create the aws key pair and remote provision the EC2 instance | `string` | n/a | yes |
| <a name="input_instance_type"></a> [instance\_type](#input\_instance\_type) | Moxy instance type | `string` | n/a | yes |
| <a name="input_moxy_bin_path"></a> [moxy\_bin\_path](#input\_moxy\_bin\_path) | Moxy path to binary to copy to the worker machine | `string` | n/a | yes |
| <a name="input_tags"></a> [tags](#input\_tags) | Optional set of tags to use for all resources | `map(string)` | `{}` | no |
| <a name="input_vpc_id"></a> [vpc\_id](#input\_vpc\_id) | VPC ID to provision the EC2 instance | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_moxy_password"></a> [moxy\_password](#output\_moxy\_password) | Moxy server password |
| <a name="output_moxy_url"></a> [moxy\_url](#output\_moxy\_url) | Moxy server URL |
<!-- END_TF_DOCS -->