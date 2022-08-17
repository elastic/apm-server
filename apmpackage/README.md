## Developer documentation

### Guide

#### Update / fix a package

1. Modify integration package
    - Make changes in `apmpackage/apm` and/or code as needed
    - Run `make check-package` to check your package passes linting rules

2. Run the stack
    - Run `docker-compose up -d`: this will start Elasticsearch and Kibana.

3. Test changes
    - If you just want to intall the integration package, you can do so by running `cd systemtest && go run ./cmd/runapm -init`.
      This will build the integration package and upload it to Kibana for installation.
    - If you want to also test code changes in conjunction with changes to the integration package, you can do so by running
      `cd systemtest && go run ./cmd/runapm`. In addition to the above, this will build apm-server and inject it into a local
      Elastic Agent Docker image, start a container running the image, and configure it with an APM integration policy.

Once changes have been merged into the main branch, the integration package will be built by CI and published to package storage.
