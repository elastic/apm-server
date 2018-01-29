# APM Server-Testing

## Automated Testing
The tests are built on top of the [Beats Test Framework](https://github.com/elastic/beats/blob/master/docs/devguide/testing.asciidoc), where you can find a detailed description on how to run the test suite.

### Quick Overview
Run the full test suite of APM Server, which needs all dependencies to run locally or in a Docker environment:

```
make testsuite
```

Only run unit tests without external dependencies:

```
make unit
```

## Coverage Report
For insights about test-coverage, run `make coverage-report`. The test coverage is reported in the folder `./build/coverage/`

## Snapshot-Testing
Some tests make use of the concept of _snapshot_ or _approvals testing_. If running tests leads to changed snapshots, you can use the `approvals` tool to update the snapshots.
Following workflow is intended:
* Run `make update` to create the `approvals` binary that supports reviewing changes. 
* Run `make unit` to create a `*.received.json` file for every newly created or changed snapshot.
* Run `./approvals` to review and interactively accept the changes. 

## Manual Testing

For manual testing with the elastic stack there is a test environment based on docker. 
To get this running execute the following commands.

* Run `STACK_VERSION=6.1.2 make start-env` to start docker containers for the elastic stack. 
Set the version you want to test with via the environment variable `STACK_VERSION`.
* Run `make import-dashboards` to import the Elasticsearch template and the Kibana dashboards
* Check instructions at the 
[Elastic APM agents docs](https://www.elastic.co/guide/en/apm/agent/index.html) 
to setup an agent of your choice and create test data.
Alternatively you can send a `curl` request to the 
[transaction](https://www.elastic.co/guide/en/apm/server/current/transaction-api.html#transaction-api-examples) or the 
[error](https://www.elastic.co/guide/en/apm/server/current/error-api.html#transaction-api-examples) endpoint   
* Open `localhost:5601` in your browser
* In Kibana go to the Dashboards to see APM Data
* In Kibana go to the Discovery tab to query for APM Data
