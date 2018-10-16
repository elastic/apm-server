#!/usr/bin/env bash
set -euox pipefail

source ./_beats/dev-tools/jenkins_release.sh

go test -v tests/packaging/package_test.go -files /Users/gil/.local/go/src/github.com/elastic/apm-server/build/distributions/\* -tags=package
make package-tests
