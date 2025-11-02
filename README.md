[![ci](https://github.com/elastic/apm-server/actions/workflows/ci.yml/badge.svg)](https://github.com/elastic/apm-server/actions/workflows/ci.yml)
[![Smoke Tests](https://github.com/elastic/apm-server/actions/workflows/smoke-tests-os-sched.yml/badge.svg)](https://github.com/elastic/apm-server/actions/workflows/smoke-tests-os-sched.yml)
[![Package status](https://badge.buildkite.com/fc4aa824ffecf245db871971507275aa3c35904e380fef449c.svg?branch=main)](https://buildkite.com/elastic/apm-server-package)

# APM Server

The APM Server receives data from Elastic APM agents and transforms it into Elasticsearch documents.
Read more about Elastic APM at [elastic.co/apm](https://www.elastic.co/apm).

For questions and feature requests, visit the [discussion forum](https://discuss.elastic.co/c/apm).

## Getting Started

To get started with APM, see our [Quick start guide](https://www.elastic.co/guide/en/apm/guide/current/apm-quick-start.html).

## APM Server Development

### Requirements

* [Go][golang-download]

[golang-download]: https://golang.org/dl/

### Install

* Fork the repo with the GitHub interface and clone it:

```
git clone git@github.com:[USER]/apm-server.git
```

Note that it should be cloned from the fork (replace [USER] with your GitHub user), not from origin.

* Add the upstream remote:

```
git remote add elastic git@github.com:elastic/apm-server.git
```

### Build

To build the binary for APM Server run the command below. This will generate a binary
in the same directory with the name apm-server.

```
make
```

If you make code changes, you may also need to update the project by running the additional command below:

```
make update
```

### Run

To run APM Server with debugging output enabled, run:

```
./apm-server -c apm-server.yml -e -d "*"
```

APM Server expects index templates, ILM policies, and ingest pipelines to be set up externally.
This should be done by [installing the APM integration](https://www.elastic.co/guide/en/observability/current/traces-get-started.html#add-apm-integration).
When running APM Server directly, it is only necessary to install the integration and not to run an Elastic Agent.

#### Tilt

You can also run APM Server in a containerized environment using
[Tilt](https://tilt.dev/).

```
tilt up
```

See [dev docs
testing](https://github.com/elastic/apm-server/blob/5f247b3f66b0fab04381eee5a53e676dba030937/dev_docs/TESTING.md#tilt--kubernetes)
for additional information.

### Testing

For Testing check out the [testing guide](dev_docs/TESTING.md)

### Cleanup

To clean up the build directory and generated artifacts, run:

```
make clean
```

### Contributing

See [contributing](CONTRIBUTING.md) for details about reporting bugs, requesting features,
or contributing to APM Server.

### Releases

See [releases](dev_docs/RELEASES.md) for an APM Server release checklist.

## Updating dependencies

APM Server uses Go Modules for dependency management, without any vendoring.

In general, you should use standard `go get` commands to add and update modules. The one exception to this
is the dependency on `libbeat`, for which there exists a special Make target: `make update-beats`, described
below.

### Updating libbeat

By running `make update-beats` the `github.com/elastic/beats/vN` module will be updated to the most recent
commit from the main branch, and a minimal set of files will be copied into the apm-server tree.

You can specify an alternative branch or commit by specifying the `BEATS_VERSION` variable, such as:

```
make update-beats BEATS_VERSION=7.x
make update-beats BEATS_VERSION=f240148065af94d55c5149e444482b9635801f27
```

## Packaging

To build all apm-server packages from source, run:

```
make package
```

This will fetch and create all images required for the build process. The whole process can take several minutes.
When complete, packages can be found in `build/distributions/`.

### Building docker packages

To customize image configuration, see [the docs](https://www.elastic.co/guide/en/apm/guide/current/running-on-docker.html).

To build docker images from source, run:

```
make package-docker
```

When complete, Docker images can be found at `build/distributions/*.docker.tar.gz`,
and the local Docker image IDs are written at `build/docker/*.txt`.

Building pre-release images can be done by running `make package-docker-snapshot` instead.

## Documentation

Documentation for the APM Server can be found in the [Observability guide's APM section](https://www.elastic.co/guide/en/observability/master/apm.html). Most documentation files live in the [elastic/observability-docs](https://github.com/elastic/observability-docs) repo's [`docs/en/observability/apm/` directory](https://github.com/elastic/observability-docs/tree/main/docs/en/observability/apm).

However, the following content lives in this repo:

* The **changelog** page listing all release notes is in [`CHANGELOG.asciidoc`](/CHANGELOG.asciidoc).
* Each minor version's **release notes** are documented in individual files in the [`changelogs/`](/changelogs/) directory.
* A list of all **breaking changes** are documented in [`changelogs/all-breaking-changes.asciidoc`](/changelogs/all-breaking-changes.asciidoc).
* **Sample data sets** that are injected into the docs are in the [`docs/data/`](/docs/data/) directory.
* **Specifications** that are injected into the docs are in the [`docs/spec/`](/docs/spec/) directory.



