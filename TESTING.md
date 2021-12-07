# APM Server-Testing

## Automated Testing
The tests are built on top of the [Beats Test Framework](https://github.com/elastic/beats/blob/master/docs/devguide/testing.asciidoc), where you can find a detailed description on how to run the test suite.

### Quick Overview

To run the unit tests, you can use `make test` or simply `go test ./...`. The unit tests do not require any external services.

The APM Server "system tests" run the APM Server in various scenarios, with the Elastic Stack running inside Docker containers.
To run the system tests locally, you can run `go test` inside the systemtest directory.

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
$ git checkout master
$ make bench > old.txt
$ benchcmp old.txt new.txt
```
