#!/usr/bin/env bash
set -exuo pipefail

source ./script/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
  .ci/scripts/docker-get-logs.sh
}
trap cleanup EXIT

make update apm-server

# Start docker-compose environment first, so it doesn't count towards the test timeout.
${PYTHON_ENV}/linux/bin/docker-compose up -d

OUTPUT_DIR="$(pwd)/build"
OUTPUT_JSON_FILE="$OUTPUT_DIR/TEST-go-system_tests.out.json"
OUTPUT_JUNIT_FILE="$OUTPUT_DIR/TEST-go-system_tests.xml"

# Download systemtest dependencies soo the download time doesn't count towards
# the test timeout.
cd systemtest && go mod download && cd -

export GOTESTFLAGS="-v -json"
gotestsum --no-color -f standard-quiet --jsonfile "$OUTPUT_JSON_FILE" --junitfile "$OUTPUT_JUNIT_FILE" --raw-command -- make system-test
