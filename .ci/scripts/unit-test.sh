#!/usr/bin/env bash
set -exo pipefail

source ./script/common.bash

jenkins_setup

make update
mage goTestUnit

OUT_FILE="build/TEST-go-unit"
if [ -n "${TEST_COVERAGE}" ] && [ -f "${OUT_FILE}.cov" ]; then
  go install -modfile=tools/go.mod github.com/t-yuki/gocover-cobertura
  gocover-cobertura < "${OUT_FILE}.cov" > "${OUT_FILE}_cov.xml"
fi
