#!/usr/bin/env bash
set -exuo pipefail

source ./script/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
  .ci/scripts/docker-get-logs.sh
}
trap cleanup EXIT

make update docker-system-tests

# TODO(axw) make this part of the "system-tests" target
(cd systemtest && go test -v ./...)
