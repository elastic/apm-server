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

For manual testing with the elastic stack and the agents there is a test environment based on docker. To
get this running execute the following commands.

* Run `make start-env`
* Run 3 times around your table, it takes ES quite a bit to startup completely with X-Pack
* Go to `localhost:5000` or `localhost:5000/error` and press refresh a few times
* Open `localhost:5601` in your browser and log into Kibana with `elastic` and `changeme`
* Create the `apm-server-*` index pattern
* In Kibana go to the Discovery tab and you should see data

For manual testing with specific agents, check instructions at `tests/agent/[LANG]/README.md`

