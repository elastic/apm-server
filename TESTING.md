# Apm-Server-Testing

To run the full testsuite of Apm-Server, run:

```
make testsuite
```


Alternatively only specific tests can be run:

```
Make unit
```

Runs tests not relying on external dependencies. If tests coverage should be printed, run `make unit-tests` instead.

```
make integration-tests
```

Runs tests labeled as integration tests. Integration tests can depend on external systems. If tests should be run inside a virtual environment, run `make integration-tests-environment`. This can be run on any docker-machine (local, remote). If no environment is attached only tests are run which do not require an environment. 

```
make system-tests
```

Runs end-to-end system tests, that depend on externals systems. If tests should be run inside a virtual environment, run `make system-tests-environment`. This can be run on any docker-machine (local, remote). If no environment is attached only tests are run which do not require an environment. If you want to run system tests with your local environment, you would have to run `INTEGRATION_TESTS=1 make system-tests`.

## Coverage Report
For insights about test-coverage, run `make coverage-report`. The test coverage is reported in the folder `./build/coverage/`


## Snapshot-Testing
Some tests make use of the concept of snapshot testing. If those tests were run and the snapshot changed, the `approvals` tool can be used to to update the snapshots. Following workflow is intended to approve newly created or changed snapshots.
* running tests as described above will create a `*.received.json` file for every newly created of changed snapshot.
* run `make update` to create the `approvals` binary that supports viewing the changes. 
* run `./approvals` to review and interactively accept the changes. 
