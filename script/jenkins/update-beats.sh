#!/usr/bin/env bash
set -euox pipefail

: "${HOME:?Need to set HOME to a non-empty value.}"
: "${WORKSPACE:?Need to set WORKSPACE to a non-empty value.}"

source ./_beats/dev-tools/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
  make stop-environment fix-permissions
}
trap cleanup EXIT

go get -u github.com/kardianos/govendor
RACE_DETECTOR=1 make update-beats

eval "$(gvm $(cat ./_beats/.go-version))"

RACE_DETECTOR=1 make clean check testsuite apm-server
