# APM Server

The APM Server receives data from the Elastic APM agents and stores the data into Elasticsearch.

[Read more about Elastic APM](https://www.elastic.co/solutions/apm).

Please take questions or feedback to the [Discuss forum](https://discuss.elastic.co/c/apm) for APM.

## Getting Started

To get started with APM please see our [Getting Started Guide](https://www.elastic.co/guide/en/apm/get-started).

## APM Server Development

### Requirements

* [Golang](https://golang.org/dl/) 1.9.4

### Install

+ Fork the repo with the Github interface and clone it:

```
cd ${GOPATH}/src/github.com/elastic/
git clone git@github.com:[USER]/apm-server.git
```
Note that it should be cloned from the fork (replace [USER] with your Github user), not from origin.

+ Add the upstream remote:
```git remote add elastic git@github.com:elastic/apm-server.git```

### Build

To build the binary for APM Server run the command below. This will generate a binary
in the same directory with the name apm-server.

```
make
```

You also need to create all files needed by the APM Server by running the additional command below. 

```
make update
```
Note that this requires to have `virtualenv` installed.

### Run

To run APM Server with debugging output enabled, run:

```
./apm-server -c apm-server.yml -e -d "*"
```

### Testing

For Testing check out the [testing guide](TESTING.md)

### Update

Each beat has a template for the mapping in elasticsearch and a documentation for the fields
which is automatically generated based on `fields.yml`.
To generate required configuration files and templates run:

```
make index-template update
```

### Cleanup

To clean APM Server source code, run the following commands:

```
make fmt
```

To clean up the build directory and generated artifacts, run:

```
make clean
```

For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).

### Contributing

See [contributing](CONTRIBUTING.md) for details about reporting bugs or requesting features in APM server.

## Update Dependencies

The `apm-server` has two types of dependencies, 
the Golang packages managed with *Govendor* and a dependency to the *Beats Framework*.

### Govendor

Checkout the [govendor tool](https://github.com/kardianos/govendor).

To update beats to the most recent version from your go path for example use: `govendor fetch github.com/elastic/beats/...`.
Govendor will automatically pick the files needed.

### Beats Framework Update

To update the beats framework run `make update-beats`. This will fetch the most recent version of beats from master and copy
the files which are needed for the framework part to the `_beats` directory. These are files like libbeat config files and
scripts which are used for testing or packaging.

It is recommended to keep the version of the beats framework and libbeat in sync.
To make an update of both, run:

```
make update-beats
```

To update the dependency to a specific commit or branch run command as following:

```
BEATS_VERSION=f240148065af94d55c5149e444482b9635801f27 make update-beats
```

## Packaging

The beat frameworks provides tools to crosscompile and package your beat for different platforms. This requires [docker](https://www.docker.com/) and vendoring as described above. To build packages of your beat, run the following command:

```
make package
```

This will fetch and create all images required for the build process. The whole process can take several minutes.

## Documentation
The [Documentation](https://www.elastic.co/guide/en/apm/server/current/index.html) for the APM Server can be found in the `docs` folder.

## Help

`make help`
