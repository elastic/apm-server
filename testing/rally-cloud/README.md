# Elasticsearch cluster benchmarking with Elastic Cloud deployment

This module makes it possible to run benchmarks on Elasticsearch cluster deployed as
Elastic Cloud deployment with a locally built APM Server binary and APM integration package.
The module is created to be used by developers to benchmark changes in Elasticsearch schema
as used by APM-Server.

## How to use?

1. Navigate to `testing/cloud` and run `make` to build and push docker image for APM Server
and APM integration package.
2. Navigate to `testing/rally-cloud` and run `make init` if you are running for the first
time
3. Get `EC_API_KEY` by following [this guide](https://www.elastic.co/guide/en/cloud-enterprise/current/ece-restful-api-authentication.html#ece-api-keys).
4. Checkout the `testing/rally-cloud/variables.tf` to customize your benchmark.
5. Run `EC_API_KEY=<ec_api_key> make apply` to create the required infra and run rally.

The rally benchmark will execute everytime `make apply` is called however, the infrastucture
will be created only once. After testing is done make sure to destroy all infrastructure using
`EC_API_KEY=<ec_api_key> make destroy`
