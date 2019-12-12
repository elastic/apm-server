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

### Developing Tests

While developing new tests or troubleshooting test failures, it is handy to run tests from outside of docker, for
example from within an editor, while still allowing all dependencies to run in containers.  To accomplish this:

* Run `make start-environment` to start docker containers for the Elastic Stack.
* Run `PYTHON_EXE=python2.7 make python-env` to build a python virtualenv
* Run tests using the `run-system-tests` target, eg:
 ```
 SYSTEM_TEST_TARGET=./tests/system/test_integration.py:SourcemappingIntegrationTest.test_backend_error make run-system-test
```

## Coverage Report
For insights about test-coverage, run `make coverage-report`. The test coverage is reported in the folder `./build/coverage/`

## Snapshot-Testing
Some tests make use of the concept of _snapshot_ or _approvals testing_. If running tests leads to changed snapshots, you can use the `approvals` tool to update the snapshots.
Following workflow is intended:
* Run `make update` to create the `approvals` binary that supports reviewing changes. 
* Run `make unit` to create a `*.received.json` file for every newly created or changed snapshot.
* Run `./approvals` to review and interactively accept the changes. 

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
$ git checkout master
$ make bench > old.txt
$ benchcmp old.txt new.txt
```
