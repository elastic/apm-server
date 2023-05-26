# Benchmark environment

**NOTE: This is unlikely to be useful for non-Elastic employees**

This folder contains the necessary scripts to create a customizable deployment
with APM Server and an AWS worker VM with `apmbench` uploaded with the necessary
credentials to run the benchmark against it.

## Dependencies

- `terraform`
- `curl`
- `ssh`
- `ssh-keygen`
- `scp`
- `jq`
- `go`
- `okta-awscli` (only for `all` and `auth` targets)
  - `python3`
  - `awscli`

## Credentials

To successfully use the this environment, you will need to have credentials for:

- An AWS account.
- ESS or self-serviced ECE environment API key (`EC_API_KEY` environment variable).

### AWS Credentials

In order to facilitate self-service of this environment without assuming the consumer
has the AWS credentials up to date, the `make all` (default target) and `make auth`
uses `okta-awscli` to generate the necessary credentials in `~/.aws/credentials`.
For more information on how to set up these, please refer to our [internal docs](https://github.com/elastic/observability-dev/blob/main/environments/ecetest/README.md#okta-aws-cli-setup).

It isn't a requirement to use `okta-awscli`, and your own credentials can be used
as long as they are provided in `~/.aws/credentials` under the `default` profile.

If you want to use a different profile, you can set the `AWS_PROFILE` make variable
to a different value.

### ESS Credentials

In order for the `ec_deployment` provider to work against ESS, it is necessary to provide
an API Key by exporting the environment variable `EC_API_KEY`. To generate a new API
key in ESS, you can navigate to [Features > API Keys > Generate API Key](https://cloud.elastic.co/deployment-features/keys).

## Usage

The `make` command is meant to be the main entrypoint to this environment, providing a light
layer of abstraction over terraform and other utility commands to facilitate the interaction.

Please ensure that you have both the ESS / ECE and AWS credentials. `make auth` will make sure
that you're authenticated with AWS, but ESS is left to configure on your own.

The main commands are:

- `all` (default): runs `auth`, `apmbench`, creates the config files and runs terraform apply.
- `auth`: Re-generate AWS credentials, they will expire after 4h.
- `run-benchmark`: Run the benchmarks, can configured by tweaking:
  - `BENCHMARK_WARMUP_TIME`: Set the amount of time to warm the APM Server for. Defaults to `5m`.
  - `BENCHMARK_AGENTS`: Set the number of agents to send data to the APM Server. Defaults to `64`.
  - `BENCHMARK_COUNT`: Set the number of times each benchmark scenario is run. Defaults to `3`.
  - `BENCHMARK_TIME`: Set the amount of time to run each benchmark scenario for. Defaults to `2m`.
  - `BENCHMARK_RUN`: Set the expression that matches the benchmark scenarios to run. Defaults to `Benchmark` (all).
  - `BENCHMARK_RESULT`: Set the output file where the results of the benchmark will be written. Defaults to `benchmark-result.txt`
  - `BENCHMARK_DETAILED`: Sets the `-detailed` when running `apmbench`, displaying extra metrics for each benchmark. Defaults to `false`.
- `index-benchmark-result`: Indexes `$(BENCHMARK_RESULT)` to an Elasticsearch cluster. Can be configured with:
  - `GOBENCH_INDEX`: Set the Elasticsearch index where the benchmark results will be stored. Defaults to `gobench`.
  - `GOBENCH_USERNAME`: Set the Elasticsearch username to use for authentication. Defaults to `admin`.
  - `GOBENCH_PASSWORD`: Set the Elasticsearch password to use for authentication. Defaults to `changeme`.
  - `GOBENCH_HOST`: Set the Elasticsearch host where the results will be indexed Defaults to `http://localhost:9200`.
  - `GOBENCH_TAGS`: Set additional tags to include in the Elasticsearch documents. No default.

Helper commands

- `~/.ssh/id_rsa_terraform`: Generates a new SSH key without passphrase for the worker VMs.
- `terraform.tfvars`: Copies the examples tfvars and sets the `user_name` var with to `$USER`.
- `apmbench`: Compiles the `apmbench` binary from the provided location (`APMBENCH_PATH`).

### Override the docker image  and image tag

Running `make docker-override-committed-version` will create new docker images for `kibana` and `elastic-agent`
with local `apm` package and `apm-server` and a Terraform variable file. The file named 
`docker_image.auto.tfvars` contains Terraform Docker image Terraform variables overrides. This file is not 
overridden automatically, you need to remove it manually if present.

#### Override docker image tag

It is possible to override the tag of the docker image that is run in the remote ESS deployment. You can
specify any of the avilable tags (such as `8.3.0-SNAPSHOT` or a more specific tag `8.3.0-c655cda8-SNAPSHOT`).
Alternatively, you can run `make docker-override-committed-version` in your shell, to have use the committed
tags in the `docker-compose.yml` file in the repository root.

#### Override the docker image

It is also possible to override the docker image to one that is allowed to run in ESS. For more information
on which repositories can be used, please refer to our internal docs. To override the docker image, you'll need
to specify the full object of images that is defined in `variables.tf`: `docker_image_override`.
Alternatively, you can run `make docker-override-committed-version` in your shell, to have use the committed
tags in the `docker-compose.yml` file in the repository root.

### Set APM index shards

By default, the APM indices ship with `number_of_shards` set to `1`. To override this behavior, you can modify the
`apm_shards` variable and individually set the setting for each of the component templates. See an example of how to
do that in `terraform.tfvars.example`.

### Delete all the APM data streams

`make cleanup-elasticsearch` will delete all the APM data streams. This may be useful in case you'd like to re-run
the benchmarks without destroying the deployment.

### Slack reporting
Reporting data is taken from the https://`<replace-with-kibana-benchmark-url>`/app/dashboards#/view/a5bc8390-2f8e-11ed-a369-052d8245fa04?_g=(filters:!(),refreshInterval:(pause:!t,value:0),time:(from:now-30d,to:now))
It's possible to add or modify any metric.

Naming convention for mertics: `[metic_name]_(1w|2w|3w)`. The slack message contains an image and metric details (when CSV reporting is back in Kibana)