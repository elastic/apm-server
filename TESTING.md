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

## Local testing

Often, we need to manually test the integration between different features, PR testing or pre-release testing.
Our `docker-compose.yml` contains the basic components that make up the Elastic Stack for the APM Server.

### Testing Stack monitoring

APM Server publishes a set of metrics that are consumed consumed either by Metricbeat or sent by the APM Server
to an Elasticsearch cluster. Some of these metrics are used to power the Stack Monitoring UI. The stack monitoring
setup is non trivial and has been automated in `testing/stack-monitoring.sh`. The script will launch the necessary
stack components, modify the necessary files and once finished, you'll be able to test or ensure that Stack Monitoring
is working as expected.

### Injecting an APM Server binary into Elastic Agent

Since APM Server is now run by the Elastic Agent in managed mode, only testing the APM Server in Standalone mode will
not completely test the supported and recommended APM Server setup. To reuse the `docker-compose.yml` components and
ease testing, you can inject an `apm-server` binary in the Elastic Agent container so a locally built version can be
tested while making changes to the APM Server or before a release is published.

To do so, you can build the binary locally with and copy the apm-server binary to the folder that is bindmounted in the
Elastic Agent docker container:

```console
$ GOOS=linux make
$ cp apm-server testing/docker/fleet-server
```

The workflow that needs to be followed is:

1. Build the APM Server and copy it to the expected folder (As shown above).
2. `docker-compose up -d`.
3. Once everything is up and running, make sure the APM Integration is installed and assigned to the Fleet Policy.
4. Restart the `fleet-server` container: `docker-compose restart fleet-server`.

We need to restart the Elastic Agent container after the APM Server as extracted by the ELastic Agaent has been run
at least once, since we rely on the paths to be created, otherwise, the binary will be overwritten by Elastic Agent.

Alternatively, run `testing/stack-monitoring.sh`. The script follows a similar workflow and install the APM Integration
by default, so you don't have to worry about the details.
