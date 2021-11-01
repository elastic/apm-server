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

SYSTEM_TESTS_XUNIT_PATH="$(pwd)/build"
# TODO(axw) make this part of the "system-tests" target
# TODO(mdelapenya) meanwhile there is no 'DefaultGoTestSystemArgs' at beats' GoTest implementation, this command is reproducing what Beats should provide: a gotestsum representation for system tests.
#    In the same hand, the system tests should be tagged with "// +build systemtests"
(cd systemtest && gotestsum --no-color -f standard-quiet --junitfile "${SYSTEM_TESTS_XUNIT_PATH}/TEST-go-system_tests.xml" --jsonfile "${SYSTEM_TESTS_XUNIT_PATH}/TEST-go-system_tests.out.json" -- -tags  systemtests ./...)
