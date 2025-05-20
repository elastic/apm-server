[![Functional Tests](https://github.com/elastic/apm-server/actions/workflows/functional-tests.yml/badge.svg)](https://github.com/elastic/apm-server/actions/workflows/functional-tests.yml)

# Functional Tests

The functional tests test that APM Server works as expected after version upgrades.

## Running the Tests

To run the tests, you will first need to set the `EC_API_KEY` environment variable, which can be obtained by following
[this guide](https://www.elastic.co/guide/en/cloud/current/ec-api-authentication.html).

Then, from the current directory, simply run:
```sh
go test -v -timeout=30m -cleanup-on-failure=false -target="pro" ./
```

You can also specify a specific test you want to run, for example:
```sh
go test -run=TestUpgrade_8_18_to_9_0 -v -timeout=30m -cleanup-on-failure=false -target="pro" ./
```

Note: Before running tests, make sure to delete the Terraforms by running `rm -r tf-*`.

### Debugging the Tests

If you get some errors after running the test, you can try heading to the [Elastic Cloud console](https://cloud.elastic.co/home)
in order to access the Kibana instance. From there, you can use Dev Tools to check the data streams etc.

Note: If the tests failed due to deployment, you may need to access the Elastic Cloud admin console instead to check the
deployment errors.

## Code Structure

The following is the simplified directory structure of functional tests.
```
- functionaltests/
   |- infra/
   |- internal/
   |- tests/
       |- standalone/
       |- deep/
```

All the functional tests are written in the `tests/` directory. The functional tests are separated in two different types
of tests, deep upgrade tests in `tests/deep/` and standalone-to-managed tests in `tests/standalone/`.

The `internal/` directory contains helper packages used in the tests, e.g. Elasticsearch, Kibana client wrapper etc.

The `infra/` directory contains infrastructure related code. In our case, we use Terraform for deploying the stack in
Elastic Cloud. The Terraform files are located in `infra/terraform`, and are copied into `tf-<test_name>/` in the working 
directory e.g. `tests/deep/tf-TestUpgrade_8_19_to_9_0/`, at the start of each test (since Terraform saves state in the directory 
it is initialized in).

### Deep Upgrade Tests

Deep upgrade tests checks that APM Server works properly across upgrade in specific circumstances. These include checking
for document counts, data stream / indices lifecycle etc.

We suggest each deep upgrade test to be named in the format of `TestUpgrade_<from_version>_to_<to_version_1>[_to_<to_version_N>]*[_<suffix>]?`.
This means that the test will start from `from_version`, and be upgraded to `to_version_1`, then subsequently to
`to_version_2` etc. all the way to `to_version_N`.

The upgrade tests are implemented in each version test file. The test file is named after the first version of the upgrade
chain. For example, `TestUpgrade_8_15_to_8_16` will be in `8_15_test.go`.

### Standalone-to-Managed Tests

Standalone-to-managed tests checks that APM Server works properly after being converted from standalone to Fleet managed.

The 3 tests are:
1. 7.x standalone -> 8.x standalone -> 8.x managed -> 9.x managed
2. 7.x standalone -> 8.x standalone -> 9.x standalone -> 9.x managed
3. 7.x standalone -> 7.x managed -> 8.x managed -> 9.x managed

The standalone-to-managed tests are implemented in `standalone_test.go`.