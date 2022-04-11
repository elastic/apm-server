# APM Server-Testing

## Automated Testing

The tests are built on top of the [Beats Test Framework](https://github.com/elastic/beats/blob/main/docs/devguide/testing.asciidoc), where you can find a detailed description on how to run the test suite.

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
$ git checkout main
$ make bench > old.txt
$ benchcmp old.txt new.txt
```

## Manual testing

Often, we need to manually test the integration between different features, PR testing or pre-release testing.
Our `docker-compose.yml` contains the basic components that make up the Elastic Stack for the APM Server.

### Testing Stack monitoring

APM Server publishes a set of metrics that are consumed either by Metricbeat or sent by the APM Server to an
Elasticsearch cluster. Some of these metrics are used to power the Stack Monitoring UI. The stack monitoring
setup is non trivial and has been automated in `testing/stack-monitoring.sh`. The script will launch the
necessary stack components, modify the necessary files and once finished, you'll be able to test or ensure
that Stack Monitoring is working as expected.

Note that the `testing/stack-monitoring.sh` script relies on `systemtest/cmd/runapm`, and will use a locally built
version of APM Server (see more information below).

### Injecting an APM Server binary into Elastic Agent

APM Server can be run in either standalone or managed mode by the ELastic Agent. To facilitate manual testing of
APM Server in managed mode, it is possible to inject a locally built `apm-server` binary via `systemtest/cmd/runpm`.
It requires having the apm-server docker-compose project running and creates the required fleet policies, and exposes
the APM Server port using a random binding that is printed to the standard output after the container has started.

```console
$ cd systemtest/cmd/runapm
$ go run main.go -h
Usage of /var/folders/35/r4w8sbqj2md1sg944kpnzyth0000gn/T/go-build3644709196/b001/exe/main:
  -d    If true, runapm will exit after the agent container has been started
  -f    Force agent policy creation, deleting existing policy if found
  -keep
        If true, agent policy and agent will not be destroyed on exit
  -name string
        Docker container name to use, defaults to random
  -namespace string
        Agent policy namespace (default "default")
  -policy string
        Agent policy name (default "runapm")
  -reinstall
        Reinstall APM integration package (default true)
  -var value
        Define a package var (k=v), with values being YAML-encoded; can be specified more than once
```
