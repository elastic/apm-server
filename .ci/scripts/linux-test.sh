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

# Ensure our CI machines have a modern version of docker-compose.
# The Python venv is included in $PATH by script/common.bash.
make venv
pip install docker-compose==1.29.2

# Start docker-compose environment first, so it doesn't count towards the test timeout.
docker-compose up -d

OUTPUT_DIR="$(pwd)/build"
OUTPUT_JSON_FILE="$OUTPUT_DIR/TEST-go-system_tests.out.json"
OUTPUT_JUNIT_FILE="$OUTPUT_DIR/TEST-go-system_tests.xml"

# Download systemtest dependencies soo the download time doesn't count towards
# the test timeout.
cd systemtest && go mod download && cd -

export GOTESTFLAGS="-v -json"
go run -modfile=tools/go.mod gotest.tools/gotestsum \
	--no-color -f standard-quiet --jsonfile "$OUTPUT_JSON_FILE" --junitfile "$OUTPUT_JUNIT_FILE" \
	--raw-command -- make system-test
