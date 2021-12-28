#!/usr/bin/env bash
set -exuo pipefail

source ./script/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
  .ci/scripts/docker-get-logs.sh
}
trap cleanup EXIT

make update apm-server docker-system-tests

<<<<<<< HEAD
SYSTEM_TESTS_XUNIT_PATH="$(pwd)/build"
# TODO(axw) make this part of the "system-tests" target
# TODO(mdelapenya) meanwhile there is no 'DefaultGoTestSystemArgs' at beats' GoTest implementation, this command is reproducing what Beats should provide: a gotestsum representation for system tests.
#    In the same hand, the system tests should be tagged with "// +build systemtests"
(cd systemtest && gotestsum --no-color -f standard-quiet --junitfile "${SYSTEM_TESTS_XUNIT_PATH}/TEST-go-system_tests.xml" --jsonfile "${SYSTEM_TESTS_XUNIT_PATH}/TEST-go-system_tests.out.json" -- -tags  systemtests ./...)
=======
# Start docker-compose environment first, so it doesn't count towards the test timeout.
docker-compose up -d

OUTPUT_DIR="$(pwd)/build"
OUTPUT_JSON_FILE="$OUTPUT_DIR/TEST-go-system_tests.out.json"
OUTPUT_JUNIT_FILE="$OUTPUT_DIR/TEST-go-system_tests.xml"

# Download systemtest dependencies soo the download time doesn't count towards
# the test timeout.
cd systemtest && go mod download && cd -

export GOTESTFLAGS="-v -json"
gotestsum --no-color -f standard-quiet --jsonfile "$OUTPUT_JSON_FILE" --junitfile "$OUTPUT_JUNIT_FILE" --raw-command -- make system-test
>>>>>>> bf75ef5e (ci: Run `go mod download` before systemtest (#6944))
