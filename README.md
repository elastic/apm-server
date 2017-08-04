# Apm-Server

This is the Apm-Server.

## Getting Started with Apm-Server

### Requirements

* [Golang](https://golang.org/dl/) 1.8.3

### Install

+ Fork the repo with the Github interface and clone it:

```
cd ${GOPATH}/src/github.com/elastic/
git clone https://github.com/[USER]/apm-server
```
Note that it should be cloned from the fork (replace [USER] with your Github user), not from origin.

+ Add the upstream remote:
```git remote add elastic git@github.com:elastic/apm-server.git```

### Build

To build the binary for Apm-Server run the command below. This will generate a binary
in the same directory with the name apm-server.

```
make
```

### Run

To run Apm-Server with debugging output enabled, run:

```
./apm-server -c apm-server.yml -e -d "*"
```

### Test

To test Apm-Server, run the following command:

```
make testsuite
```

alternatively:
```
make unit
make unit-tests
make system-tests
make integration-tests
make coverage-report
```

The test coverage is reported in the folder `./build/coverage/`

### Update

Each beat has a template for the mapping in elasticsearch and a documentation for the fields
which is automatically generated based on `etc/fields.yml`.
To generate etc/apm-server.template.json and etc/apm-server.asciidoc

```
make update
```

### Cleanup

To clean Apm-Server source code, run the following commands:

```
make fmt
```

To clean up the build directory and generated artifacts, run:

```
make clean
```

For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).

## Packaging

The beat frameworks provides tools to crosscompile and package your beat for different platforms. This requires [docker](https://www.docker.com/) and vendoring as described above. To build packages of your beat, run the following command:

```
make package
```

This will fetch and create all images required for the build process. The hole process to finish can take several minutes.

## Update Dependencies

The `apm-server` has two types of dependencies:

* Golang packages managed with `govendor`
* Beats framework managed with `cd _beats && sh update.sh`

It is recommended to keep the version of the beats framework and libbeat in sync. To make an update of both, run `make update-beats`.

### Govendor

For details on [govendor](https://github.com/kardianos/govendor) check the docs [here](https://github.com/kardianos/govendor).

To update beats to the most recent version from your go path for example use: `govendor fetch github.com/elastic/beats/...`.
Govendor will automatically pick the files needed.

### Framework Update

To update the beats framework run `make update-framework`. This will fetch the most recent version of beats from master and copy
the files which are needed for the framework part to the `_beats` directory. These are files like libbeat config files and
scripts which are used for testing or packaging.

## Test environment

For manual testing with the elastic stack and the agents there is a test environment based on docker. To
get this running execute the following commands.

* Run `make start-env`
* Run 3 times around your table, it takes ES quite a bit to startup completely with X-Pack
* Go to `localhost:5000` or `localhost:5000/error` and press refresh a few times
* Open `localhost:5601` in your browser and log into Kibana with `elastic` and `changeme`
* Create the `apm-server-*` index pattern
* In Kibana go to the Discovery tab and you should see data

For manual testing with specific agents, check instructions at `tests/agent/[LANG]/README.md`

## Documentation

A JSON-Schema spec for the API lives in `docs/spec`. 
ElasticSearch fields are defined in `_meta/fields.generated.yml`.
Examples of input and output documents can be found at `docs/data/intake-api` and `docs/data/elasticsearch` respectively.

## Help

`make help`
