## Soaktest worker module

This module sets up worker with load generation binary for soaktest configured as a systemd unit along with required monitoring setup.

## Pre-requisites

1. Follow the steps in [this guide](https://registry.terraform.io/providers/hashicorp/google/latest/docs/guides/getting_started).
2. The project uses [OS login](https://cloud.google.com/compute/docs/oslogin) to ssh into the created worker node, make sure that your google account has the [required privileges](https://cloud.google.com/compute/docs/oslogin/set-up-oslogin#grant-iam-roles)

<!-- BEGIN_TF_DOCS -->
## Requirements

No requirements.

## Providers

| Name | Version |
|------|---------|
| <a name="provider_google"></a> [google](#provider\_google) | n/a |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [google_compute_firewall.allow_ssh](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_firewall) | resource |
| [google_compute_instance.worker_instance](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_instance) | resource |
| [google_compute_network.apmsoak_worker_network](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/compute_network) | resource |
| [google_os_login_ssh_public_key.cache](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/os_login_ssh_public_key) | resource |
| [google_service_account.worker_svc_account](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/service_account) | resource |
| [google_client_openid_userinfo.me](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/client_openid_userinfo) | data source |
| [google_compute_image.worker_image](https://registry.terraform.io/providers/hashicorp/google/latest/docs/data-sources/compute_image) | data source |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_apm_loadgen_agents_replicas"></a> [apm\_loadgen\_agents\_replicas](#input\_apm\_loadgen\_agents\_replicas) | Number of agents replicas to use, each replica launches 4 agents, one for each type | `string` | n/a | yes |
| <a name="input_apm_loadgen_max_rate"></a> [apm\_loadgen\_max\_rate](#input\_apm\_loadgen\_max\_rate) | Max load generation rate | `string` | n/a | yes |
| <a name="input_apm_secret_token"></a> [apm\_secret\_token](#input\_apm\_secret\_token) | Secret token for auth against the given server URL | `string` | n/a | yes |
| <a name="input_apm_server_url"></a> [apm\_server\_url](#input\_apm\_server\_url) | APM Server URL for sending the generated load | `string` | n/a | yes |
| <a name="input_apmsoak_bin_path"></a> [apmsoak\_bin\_path](#input\_apmsoak\_bin\_path) | Path where the apmsoak binary resides on the local machine | `string` | n/a | yes |
| <a name="input_elastic_agent_version"></a> [elastic\_agent\_version](#input\_elastic\_agent\_version) | Version of elastic-agent to install on the monitoring worker nodes | `string` | n/a | yes |
| <a name="input_fleet_enrollment_token"></a> [fleet\_enrollment\_token](#input\_fleet\_enrollment\_token) | Fleet enrollment token to enroll elastic agent for monitoring worker nodes | `string` | n/a | yes |
| <a name="input_fleet_url"></a> [fleet\_url](#input\_fleet\_url) | Fleet URL to enroll elastic agent for monitoring worker nodes | `string` | n/a | yes |
| <a name="input_gcp_project"></a> [gcp\_project](#input\_gcp\_project) | GCP Project name | `string` | `"elastic-apm"` | no |
| <a name="input_gcp_region"></a> [gcp\_region](#input\_gcp\_region) | GCP region | `string` | `"us-west2"` | no |
| <a name="input_gcp_zone"></a> [gcp\_zone](#input\_gcp\_zone) | GCP zone | `string` | `"us-west2-b"` | no |
| <a name="input_private_key"></a> [private\_key](#input\_private\_key) | Private key to be used to establish ssh connection with the worker | `string` | n/a | yes |
| <a name="input_public_key"></a> [public\_key](#input\_public\_key) | Public key to be added to the current IAM user for OS login | `string` | n/a | yes |

## Outputs

No outputs.
<!-- END_TF_DOCS -->
