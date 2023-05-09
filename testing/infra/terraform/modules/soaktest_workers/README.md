<!-- BEGIN_TF_DOCS -->
## Soaktest worker module

This module sets up worker with load generation binary for soaktest configured as a systemd unit along with required monitoring setup. The workers generated are immutable in nature and are recreated for every configuration change.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | n/a |
| <a name="provider_tls"></a> [tls](#provider\_tls) | n/a |

## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_tags"></a> [tags](#module\_tags) | ../tags | n/a |

## Resources

| Name | Type |
|------|------|
| [google_compute_firewall.allow_ssh](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_firewall) | resource |
| [google_compute_instance.worker](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_instance) | resource |
| [google_compute_network.worker](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network) | resource |
| [google_service_account.worker](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/service_account) | resource |
| [tls_private_key.worker_login](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/resources/private_key) | resource |
| [google_compute_image.worker_image](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/compute_image) | data source |
| [tls_public_key.worker_login](https://registry.terraform.io/providers/hashicorp/tls/latest/docs/data-sources/public_key) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apm_loadgen_agents_replicas"></a> [apm\_loadgen\_agents\_replicas](#input\_apm\_loadgen\_agents\_replicas) | Number of agents replicas to use, each replica launches 4 agents, one for each type | `string` | n/a | yes |
| <a name="input_apm_loadgen_event_rate"></a> [apm\_loadgen\_event\_rate](#input\_apm\_loadgen\_event\_rate) | Load generation rate | `string` | n/a | yes |
| <a name="input_apm_loadgen_rewrite_ids"></a> [apm\_loadgen\_rewrite\_ids](#input\_apm\_loadgen\_rewrite\_ids) | Rewrite event IDs every iteration, maintaining event relationships | `bool` | `false` | no |
| <a name="input_apm_loadgen_rewrite_timestamps"></a> [apm\_loadgen\_rewrite\_timestamps](#input\_apm\_loadgen\_rewrite\_timestamps) | Rewrite event timestamps every iteration, maintaining relative offsets | `bool` | `false` | no |
| <a name="input_apm_secret_token"></a> [apm\_secret\_token](#input\_apm\_secret\_token) | Secret token for auth against the given server URL | `string` | n/a | yes |
| <a name="input_apm_server_url"></a> [apm\_server\_url](#input\_apm\_server\_url) | APM Server URL for sending the generated load | `string` | n/a | yes |
| <a name="input_apmsoak_bin_path"></a> [apmsoak\_bin\_path](#input\_apmsoak\_bin\_path) | Path where the apmsoak binary resides on the local machine | `string` | n/a | yes |
| <a name="input_elastic_agent_version"></a> [elastic\_agent\_version](#input\_elastic\_agent\_version) | Version of elastic-agent to install on the monitoring worker nodes | `string` | n/a | yes |
| <a name="input_fleet_enrollment_token"></a> [fleet\_enrollment\_token](#input\_fleet\_enrollment\_token) | Fleet enrollment token to enroll elastic agent for monitoring worker nodes | `string` | n/a | yes |
| <a name="input_fleet_url"></a> [fleet\_url](#input\_fleet\_url) | Fleet URL to enroll elastic agent for monitoring worker nodes | `string` | n/a | yes |
| <a name="input_gcp_project"></a> [gcp\_project](#input\_gcp\_project) | GCP Project name | `string` | `"elastic-apm"` | no |
| <a name="input_gcp_region"></a> [gcp\_region](#input\_gcp\_region) | GCP region | `string` | `"us-west2"` | no |
| <a name="input_gcp_zone"></a> [gcp\_zone](#input\_gcp\_zone) | GCP zone | `string` | `"us-west2-b"` | no |
| <a name="input_machine_type"></a> [machine\_type](#input\_machine\_type) | Machine type for soak test workers | `string` | `"e2-small"` | no |
<!-- END_TF_DOCS -->