# Long running soak test worker

This doc provides a guide for running soak testing as a long running tests with the required infrastructure managed by terraform. The [soaktest_workers](../infra/terraform/modules/soaktest_workers) terraform module is available to allow running [apmsoak binary](../../systemtest/cmd/apmsoak) on GCP VMs. The binary will run as systemd unit configured to restart on failure. Monitoring is also setup for the worker as part of the terraform module to monitor the node resources and the systemd unit status.

## Status of soak testing cluster

The Elastic internal soak testing infrastructure for APM Server has been decommissioned from version `8.15.2`. The associated infrastructure and alerting mechanisms have been removed and are no longer running. Other forms of testing are still in place.

## Dependencies

- `terraform`
- `gcloud CLI`

## Pre-requisites

1. Install terraform
2. If you want to persist the terraform state in a google cloud bucket then create a GCS bucket to serve as a backend (otherwise the state will be saved locally). Remote backend is useful if the setup is meant to be used by more than an individual (example: make the setup accessible to a whole team).
3. Make sure that you have access to the GCP project and have enough privileges to create and manage instances. A set of required roles to be able to create worker using this module are:
    a. `Storage Object Admin` on the backend bucket if GCS bucket is used
    b. `Compute Network Admin`
    c. `Compute Security Admin`
    d. `Create Service Accounts`
    e. `Delete Service Accounts`
    f. `Compute Instance Admin (v1)`
    g. `Service Account User` if service account is used
4. Install [gcloud CLI](https://cloud.google.com/sdk/docs/install)
5. Authenticate with GCP by running `gcloud auth application-default login`
6. Build an `apmsoak` binary. Example:
   ```
   GOOS=linux go build github.com/elastic/apm-perf/cmd/apmsoak

   ```

For full guide refer to the official getting started guide for terraform google provider [here](https://registry.terraform.io/providers/hashicorp/google/latest/docs/guides/getting_started).

## Example

```tf
terraform {
  required_version = ">= 1.1.8, < 2.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">=4.27.0"
    }
  }
  # If GCS bucket is used as a remote backend
  backend "gcs" {
    bucket = ${gcs_bucket_name}
    prefix = "/tfstate"
  }
}

module "soaktest_workers" {
  source = "../infra/terraform/modules/soaktest_workers"

  gcp_project  = ${gcp_project}
  gcp_region   = ${gcp_region}
  gcp_zone     = ${gcp_zone}
  machine_type = ${machine_type}

  apmsoak_bin_path = var.apmsoak_bin_path

  apm_server_url                 = ${apm_server_url}
  apm_secret_token               = ${apm_secret_token}
  apm_loadgen_event_rate         = ${apm_loadgen_event_rate}
  apm_loadgen_agents_replicas    = ${apm_loadgen_agents_replicas}
  apm_loadgen_rewrite_timestamps = ${apm_loadgen_rewrite_timestamps}
  apm_loadgen_rewrite_ids        = ${apm_loadgen_rewrite_ids}

  # Configure the below settings if the worker needs to be monitored
  elastic_agent_version  = ${elastic_agent_version}
  fleet_url              = ${fleet_url}
  fleet_enrollment_token = ${fleet_enrollment_token}
}
```
