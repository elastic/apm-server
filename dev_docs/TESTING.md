# APM Server Testing

## Building APM Server

APM Server can be built using `make apm-server`, which will build the apm-server binary for the host system.

`make apm-server` will build using the same flags as used in distributed artifacts, which includes stripping
debug symbols. If you wish to run APM Server under a debugger such as [`delve`](https://github.com/go-delve/delve),
you should build APM Server directly: `go build -o apm-server ./x-pack/apm-server`.

## Automated Testing

### Quick Overview

To run the unit tests, you can use `make test` or simply `go test ./...`. The unit tests do not require any external services.

The APM Server "system tests" run the APM Server in various scenarios, with the Elastic Stack running inside Docker containers.
To run the system tests locally, you can run `go test` inside the systemtest directory.

## Snapshot-Testing

Some tests make use of the concept of _snapshot_ or _approvals testing_. If running tests leads to changed snapshots,
you can run the `approvals` tool to update the snapshots. The following workflow is intended:

* Run `make test`, which will create a `*.received.json` file for every newly created or changed snapshot.
* Run `make check-approvals` to review and interactively accept the changes.

## Micro Benchmarking

To run simple benchmark tests, run:

```
make bench
```

A good way to present your results is by using `benchstat`.
With your changes in the current working tree, do:

```
$ go install golang.org/x/tools/cmd/benchstat@latest
$ make bench > new.txt
$ git checkout main
$ make bench > old.txt
$ benchstat old.txt new.txt
```

## Macro Benchmarking

Macro benchmarking focuses on measuring the APM Server's performance (throughput) and how changes in the
codebase impact that performance.

Our benchmarking tool, [`apmbench`](systemtest/cmd/apmbench), uses pre-recorded APM Agent events for the benchmarks.
This allows us to generate rich events which can be used to assess the Server's throughput under varying loads.
The [APM Integration testing](https://github.com/elastic/apm-integration-testing) framework was used to generate
the events, and [`intake-receiver`](cmd/intake-receiver/README.md) will capture the events that are sent to the
intake API. `apmbench` will records various metrics for understanding where any potential bottlenecks may be, and
how APM Server consumes the available resources.

_TODO(marclop): convert the dot diagrams from dot to mermaid so they can be read in Markdown documents_

The applications that are used to generate the stored traces may not always use the `apm-integration-testing`.
Instead, we may want to write specific applications that generate a specific type of events, rather than re-use
the existing [opbeans applications](https://github.com/elastic?q=opbeans&type=all).

### Re-generate captured events

The events are currently commited in the `apm-server` repository (`apm-server/systemtest/benchtest/events`). This
may change in the near future, and instead, we'll download the stored traces on-demand and upload/update them
periodically.

```console
# Navigate to your local copy of 'elastic/apm-integration-testing'.
$ SLEEP=180 STACK_VERSION=8.1.2 RPM=5000; ./scripts/compose.py start $STACK_VERSION --opbeans-go-loadgen-rpm ${RPM} --opbeans-python-loadgen-rpm ${RPM} --opbeans-node-loadgen-rpm $((${RPM} * 2)) --opbeans-ruby-loadgen-rpm ${RPM} --with-opbeans-go --no-apm-server-self-instrument --with-opbeans-python --with-opbeans-ruby --with-opbeans-node --apm-server-record --loadgen-no-ws && sleep $SLEEP && make copy-events; docker-compose down
...
# Copy the generated traces to the location where `apmbench` expects them to be (`apm-server/systemtest/benchtest/events`).
# Assuming that the `apm-server` repository has been checked out at the same level as `apm-integration-testing`.
$ cp -r events ../apm-server/systemtest/benchtest/events
```

### Running `apmbench`

`apmbench` is located in `systemtest/cmd/apmbench` and can target any APM Server with `apm-server.expvar.enabled`
set to `true` to be able to calculate basic throughput measurements, but `apm-server.pprof.enabled` should also
be set to `true` if any of `-blockprofile`, `-cpuprofile`, `-memprofile` or `-mutexprofile` flags are set.

The default behavior of `apmbench` is to send the captured events to the target APM Server as fast as possible
with the configured number of `-agents`. The `-agents` flag determines how many concurrent goroutines will be used
to send the events to the APM Server in parallel. The `-event-rate` can be used to specify rate of events, as `{events}/{interval}` format to send to the APM server instead of the default behaviour. For example, `1000/1s` or `10000/5s`. To benchmark the APM Server in setup similar
to what we'd see in production, the number of agents should be high (>`500`).

By default, `apmbench` will warm up the APM Server by sending events for N duration to the APM Server before any of the
benchmark scenarios are run. That N can be configured via `-warmup-time` and defaults to a conservative number of 1 minute.

The default `-benchtime` is `1s` which, for our purposes isn't a great default, so if you're benchmarking
changes to the APM Server you'll want to set the duration to at least `30s` to have some quick feedback, our
periodic benchmarks should aim to benchmark for longer to allow any long-queue effects to be detected.

The rest of the flags configure the `apmbench` so it can target an APM Server, these can be configured via the
set flags, or their `ELASTIC_APM_<UPPERCASE FLAG NAME>` alternative, for example, to configure the server URL
set `ELASTIC_APM_SERVER_URL` to the full URL of the APM Server you'd like to benchmark.

## Soak testing

Soak testing involves testing apm-server against a continuous, sustained workload to identify performance
and stability issues that occur over an extended period. `apmsoak` command can be used to generate a
sustained and continuous load for the purpose of soak testing:

```console
$ go run  github.com/elastic/apm-perf/cmd/apmsoak run -h
```

## Smoke testing

Smoke tests verify are light end to end tests which ensure that the "happy path" of the APM Server works as
expected per the asserted scenarios. The idea is to automatically gauge if there are any critical problems in
the APM Server in a regular manner.

These tests are currently using a Terraform module which manages the creation of deployments in ESS (could also
be configured to use an ECE installation) with some light bash scripting which is run in a variety of scenarios
and upgrades, but ensures there aren't any major problems with APM Server accepting and indexing events where
it should.

The smoke tests can be found under [`testing/smoke`](./../testing/smoke) and the latest CI runs can be found on 
[Github Actions](https://github.com/elastic/apm-server/actions/workflows/smoke-tests-schedule.yml).

### Debugging Failures

If the smoke tests are failing on the CI environment and active investigation is needed, it may be useful to
run the tests with the `SKIP_DESTROY` variable set to any value (`true` or `1` work too), which will prevent the
ESS deployment from being destroyed to allow some debugging to be done.

## Manual testing

Often, we need to manually test the integration between different features, PR testing or pre-release testing.
Our `docker-compose.yml` contains the basic components that make up the Elastic Stack for the APM Server.

### Tilt / Kubernetes

For local development and testing you can use [Tilt](https://tilt.dev) with a Kubernetes cluster.

We provide Kustomize manifests in [`testing/infra/k8s`](../testing/infra/k8s) for setting up
the Elastic Stack using [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html),
including standalone APM Server.
Tilt will watch for source code changes and build and inject a customized apm-server Docker image; it will
also watch for changes to the APM integration package source, and rebuild and upload the
package to Kibana on changes.

> :warning: The Tilt setup configures ECK with a trial license. By using the Tilt setup, you
> must agree to the Elastic EULA which can be found at https://www.elastic.co/eula

To use Tilt, first [install Tilt](https://docs.tilt.dev/install.html), and then create a
Kubernetes cluster. On macOS you can use Docker Desktop. On Linux you should use
[Kind](https://kind.sigs.k8s.io), optionally using [ctlptl](https://github.com/tilt-dev/ctlptl)
for managing the cluster.

Once Tilt is installed and you have a Kubernetes cluster running, simply run `tilt up` in the
root of the repository. This will present you with the option of opening a web browser to watch
changes and inspect the pod output.

When you're done, you should run `tilt down` to clean up. This will remove all resources
from the Kubernetes cluster created by Tilt.

### Testing Stack monitoring

APM Server publishes a set of metrics that are consumed either by Metricbeat or sent by the APM Server to an
Elasticsearch cluster. Some of these metrics are used to power the Stack Monitoring UI. The stack monitoring
setup is non trivial and has been automated in `testing/stack-monitoring.sh`. The script will launch the
necessary stack components, modify the necessary files and once finished, you'll be able to test or ensure
that Stack Monitoring is working as expected.

Note that the `testing/stack-monitoring.sh` script relies on `systemtest/cmd/runapm`, and will use a locally built
version of APM Server (see more information below).

### Running an Elastic Agent container with a locally built APM Server

APM Server can be run in either standalone or managed mode by the ELastic Agent. To facilitate manual testing of
APM Server in managed mode, it is possible to inject a locally built `apm-server` binary via `systemtest/cmd/runapm`.
It requires having the apm-server docker-compose project running and creates the required fleet policies, and exposes
the APM Server port using a random binding that is printed to the standard output after the container has started.

```console
$ cd systemtest/cmd/runapm
$ go run main.go -h
Usage of /var/folders/35/r4w8sbqj2md1sg944kpnzyth0000gn/T/go-build3644709196/b001/exe/main:
  -arch string
    	The architecture to use for the APM Server and Docker Image (default runtime.GOARCH)
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

### Building an Elastic Agent container image with a locally built APM Server

It's possible to run `runapm` (as pictured above) and re-use the image that `runapm` builds to use in docker-compose
files or run in ECE / ESS. However, it's also possible to only build a docker image without requiring the docker-compose
project containers to be up and running with `systemtest/cmd/buildapm`.

`buildapm` reads the `docker-compose.yml` at the root of the repository and uses that information to build an Elastic Agent
docker image with an APM Server bundled that contains any local changes you might have made.

By default, the `amd64` architecture (or platform in Docker lingo) will be used. This may not be ideal if you run a machine
with a different architecture than `amd64`, but you can specify the `-arch` flag.

Additionally, if `-cloud` is set, the Elastic Agent cloud image will be used as the base image, so changes can be packaged
and tested in ESS / ECE (See our internal documentation on these for how to use them).

```console
$ cd systemtest/cmd/buildapm
$ go run main.go -arch arm64
2022/05/05 17:50:18 Building elastic-agent-systemtest:8.3.0-e4aa1f83-SNAPSHOT (arm64) from docker.elastic.co/beats/elastic-agent:8.3.0-e4aa1f83-SNAPSHOT...
2022/05/05 17:50:18 Building apm-server...
2022/05/05 17:50:18 Built /Users/marclop/repos/elastic/apm-server/build/apm-server-linux
2022/05/05 17:50:25 Built elastic-agent-systemtest:8.3.0-e4aa1f83-SNAPSHOT (arm64)
$  go run main.go -arch amd64
2022/05/05 17:50:35 Building elastic-agent-systemtest:8.3.0-e4aa1f83-SNAPSHOT (amd64) from docker.elastic.co/beats/elastic-agent:8.3.0-e4aa1f83-SNAPSHOT...
2022/05/05 17:50:35 Building apm-server...
2022/05/05 17:50:43 Built /Users/marclop/repos/elastic/apm-server/build/apm-server-linux
2022/05/05 17:50:49 Built elastic-agent-systemtest:8.3.0-e4aa1f83-SNAPSHOT (amd64)
# go run main.go -cloud
2022/05/19 11:08:04 Building image elastic-agent-systemtest:8.3.0-e4aa1f83-SNAPSHOT (amd64) from docker.elastic.co/cloud-release/elastic-agent-cloud:8.3.0-e4aa1f83-SNAPSHOT...
2022/05/19 11:08:04 Building apm-server...
2022/05/19 11:08:04 Built /Users/marclop/repos/elastic/apm-server/build/apm-server-linux
2022/05/19 11:09:07 Built image elastic-agent-systemtest:8.3.0-e4aa1f83-SNAPSHOT (amd64)
```

### Running an Elastic Cloud deployment with a locally built APM Server and APM integration package

It is possible for Elastic employees to create an Elastic Cloud deployment with a locally built
APM Server binary and APM integration package. See [`testing/cloud`](./../testing/cloud) for instructions.
