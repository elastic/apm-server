# APM Server-Testing

## Automated Testing
The tests are built on top of the [Beats Test Framework](https://github.com/elastic/beats/blob/main/docs/devguide/testing.asciidoc), where you can find a detailed description on how to run the test suite.

### Quick Overview

To run the unit tests, you can use `make test` or simply `go test ./...`. The unit tests do not require any external services.

The APM Server "system tests" run the APM Server in various scenarios, and require an Elastic Stack to be running.
To run the system tests locally, first start Elasticsearch and Kibana, e.g.:

```
docker-compose up -d
make system-tests
```

You can alternatively run the system tests entirely within Docker:

```
make docker-system-tests
```

### Developing Tests

While developing new tests or troubleshooting test failures, it is handy to run tests from outside of docker, for
example from within an editor, while still allowing all dependencies to run in containers.  To accomplish this:

* Run `docker-compose up -d` to start docker containers for the Elastic Stack.
* Run tests with `make system-tests`, e.g.:

```
make system-tests SYSTEM_TEST_TARGET=./tests/system/test_integration.py:SourcemappingIntegrationTest.test_backend_error
```

* Or run the dockerised version of the tests with `make docker-system-tests`, e.g.:

```
make docker-system-tests SYSTEM_TEST_TARGET=./tests/system/test_integration.py:SourcemappingIntegrationTest.test_backend_error
```

Elasticsearch diagnostics may be enabled by setting `DIAGNOSTIC_INTERVAL`.
`DIAGNOSTIC_INTERVAL=1` will dump hot threads and task lists every second while tests are running
to `build/system-tests/run/$test_name/diagnostics/`.

## Snapshot-Testing
Some tests make use of the concept of _snapshot_ or _approvals testing_. If running tests leads to changed snapshots, you can use the `approvals` tool to update the snapshots.
Following workflow is intended:
* Run `make update` to create the `approvals` binary that supports reviewing changes.
* Run `make test`, which will create a `*.received.json` file for every newly created or changed snapshot.
* Run `make check-approvals` to review and interactively accept the changes.

## Benchmarking

To run simple benchmark tests, run:

```
make bench
```

A good way to present your results is by using `benchcmp`.
With your changes in the current working tree, do:

```
$ go get -u golang.org/x/tools/cmd/benchcmp
$ make bench > new.txt
$ git checkout main
$ make bench > old.txt
$ benchcmp old.txt new.txt
```
