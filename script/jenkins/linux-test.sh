#!/usr/bin/env bash
set -exuo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

#cleanup() {
#  rm -rf $TEMP_PYTHON_ENV
#  make stop-environment fix-permissions
#}
#trap cleanup EXIT

make test-deps testsuite

export COV_DIR="build/coverage"

for i in "full.cov" "integration.cov" "system.cov" "unit.cov"
do
  name=$(basename ${i} .cov)
  if [ -f "${COV_DIR}/${i}" ]; then 
    go tool cover -html="${COV_DIR}/${i}" -o "${COV_DIR}/coverage-${name}-report.html"
    gocover-cobertura < "${COV_DIR}/${i}" > "${COV_DIR}/coverage-${name}-report.xml"
  fi
done
exit 0
