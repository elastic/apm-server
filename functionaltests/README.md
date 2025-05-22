[![Functional Tests](https://github.com/elastic/apm-server/actions/workflows/functional-tests.yml/badge.svg)](https://github.com/elastic/apm-server/actions/workflows/functional-tests.yml)

# Functional Tests

The functional tests test that APM Server works as expected after version upgrades.

## Code Details

The following is a simplified directory structure of functional tests.
```
- functionaltests/
   |- infra/
   |- internal/
   |- main_test.go
   |- standalone_test.go
   |- upgrade_test.go
```

The `internal/` directory contains helper packages used in the tests, e.g. Elasticsearch, Kibana client wrapper etc.

The `infra/` directory contains infrastructure related code. In our case, we use Terraform for deploying the stack in Elastic Cloud.
The Terraform files are located in `infra/terraform`, and are copied into `tf-<test_name>/` e.g. `tf-TestUpgrade_8_19_to_9_0/`, at the start of each test (since Terraform saves state in the directory it is initialized in).

The rest are test files / utility functions for the tests.

### Upgrade Tests

The upgrade tests reside in `upgrade_test.go`.

These tests take an `upgrade-path` argument that represents a list of versions for the upgrade test.
The test will create a deployment with the first version, perform ingestion and check that everything is expected.
Then, it will consecutively upgrade to the next version, perform ingestion and check again.
For example, if we provide an `upgrade-path` of `8.15, 8.16, 8.17`, a deployment will be created in `8.15`, upgraded to `8.16` and then to `8.17`.

We provide the `upgrade-path` argument through GitHub workflow matrix, see `functional-tests.yml`.
Configuration for the upgrade test can be found in `upgrade-config.yaml`.

### Standalone-to-Managed Tests

The standalone-to-managed tests reside in `standalone_test.go`

These tests test the migration from standalone APM Server to Fleet-managed. Currently, the tests include:
- 7.x standalone -> 8.x standalone -> 8.x managed -> 9.x managed
- 7.x standalone -> 8.x standalone -> 9.x standalone -> 9.x managed
- 7.x standalone -> 7.x managed -> 8.x managed -> 9.x managed

## Running the Tests

To run the tests, you will first need to set the `EC_API_KEY` environment variable, which can be obtained by following [this guide](https://www.elastic.co/guide/en/cloud/current/ec-api-authentication.html).

### Upgrade Tests

For upgrade tests:
```sh
go test -run=TestUpgrade_UpgradePath -v -timeout=30m -cleanup-on-failure=false -target="pro" -upgrade-path="<some_upgrade_path>" ./
```

### Standalone-to-Managed Tests

For standalone tests:
```sh
go test -run=TestStandaloneManaged -v -timeout=30m -cleanup-on-failure=false -target="pro" ./
```

## Debugging the Tests

If you get some errors after running the test, you can try heading to the [Elastic Cloud console](https://cloud.elastic.co/home) in order to access the Kibana instance. 
From there, you can use Dev Tools to check the data streams etc.

Note: If the tests failed due to deployment, you may need to access the Elastic Cloud admin console instead to check the deployment errors.
