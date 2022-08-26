<!-- BEGIN_TF_DOCS -->
## Rally worker module

This modules sets up [rally daemons](https://esrally.readthedocs.io/en/stable/rally_daemon.html) which allows for distributed load generation. Such setups can generate high throughput loads and are preferred for testing large ES clusters. The module also runs `rally` with the configured arguments eact time `terraform apply` is executed.

## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_google"></a> [google](#requirement\_google) | >=4.27.0 |
| <a name="requirement_null"></a> [null](#requirement\_null) | >=3.1.1 |
| <a name="requirement_remote"></a> [remote](#requirement\_remote) | >=0.1.0 |
| <a name="requirement_tls"></a> [tls](#requirement\_tls) | >=4.0.1 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | 4.33.0 |
| <a name="provider_null"></a> [null](#provider\_null) | 3.1.1 |
| <a name="provider_remote"></a> [remote](#provider\_remote) | >=0.1.0 |
| <a name="provider_tls"></a> [tls](#provider\_tls) | 4.0.1 |

## Resources

| Name | Type |
|------|------|
| [google_compute_firewall.rally](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_firewall) | resource |
| [google_compute_instance.rally_coordinator](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_instance) | resource |
| [google_compute_instance.rally_workers](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_instance) | resource |
| [google_compute_network.rally](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network) | resource |
| [null_resource.run_rally](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [tls_private_key.rally](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/resources/private_key) | resource |
| [google_compute_image.rally](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/compute_image) | data source |
| [remote_file.rally_summary](https://registry.terraform.io/providers/tenstad/remote/latest/docs/data-sources/file) | data source |
| [tls_public_key.rally](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/data-sources/public_key) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_elasticsearch_password"></a> [elasticsearch\_password](#input\_elasticsearch\_password) | Elasticsearch password to use for benchmark with rally | `string` | n/a | yes |
| <a name="input_elasticsearch_url"></a> [elasticsearch\_url](#input\_elasticsearch\_url) | Elasticsearch URL to benchmark with rally | `string` | n/a | yes |
| <a name="input_elasticsearch_username"></a> [elasticsearch\_username](#input\_elasticsearch\_username) | Elasticsearch username to use for benchmark with rally | `string` | n/a | yes |
| <a name="input_gcp_project"></a> [gcp\_project](#input\_gcp\_project) | GCP Project name | `string` | `"elastic-apm"` | no |
| <a name="input_gcp_region"></a> [gcp\_region](#input\_gcp\_region) | GCP region | `string` | `"us-west2"` | no |
| <a name="input_gcp_zone"></a> [gcp\_zone](#input\_gcp\_zone) | GCP zone | `string` | `"us-west2-b"` | no |
| <a name="input_rally_bulk_size"></a> [rally\_bulk\_size](#input\_rally\_bulk\_size) | Bulk size to use for rally track | `number` | `5000` | no |
| <a name="input_rally_cluster_status"></a> [rally\_cluster\_status](#input\_rally\_cluster\_status) | Expected cluster status for rally | `string` | `"green"` | no |
| <a name="input_rally_dir"></a> [rally\_dir](#input\_rally\_dir) | Directory path with rally corpora and track file | `string` | n/a | yes |
| <a name="input_rally_worker_count"></a> [rally\_worker\_count](#input\_rally\_worker\_count) | Number of rally worker nodes | `number` | `2` | no |
| <a name="input_resource_prefix"></a> [resource\_prefix](#input\_resource\_prefix) | Prefix to add to all created resource | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_rally_coordinator_ip"></a> [rally\_coordinator\_ip](#output\_rally\_coordinator\_ip) | Public IP address of rally coordinator node |
| <a name="output_rally_ssh_private_key"></a> [rally\_ssh\_private\_key](#output\_rally\_ssh\_private\_key) | Private key to login to rally nodes |
| <a name="output_rally_summary"></a> [rally\_summary](#output\_rally\_summary) | Summary of rally run |
<!-- END_TF_DOCS -->