#!/usr/bin/env bash
set -euox pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

#cleanup() {
#  rm -rf $TEMP_PYTHON_ENV
#  make stop-environment fix-permissions
#}
#trap cleanup EXIT

make
make testsuite
make coverage-report

go get -v -u github.com/jstemmer/go-junit-report
go get -v -u code.google.com/p/go.tools/cmd/cover
go get -v -u github.com/t-yuki/gocover-cobertura

export COV_DIR="build/coverage"
export OUT_FILE="build/test-report.out"
mkdir -p build

go test -race ./... -v 2>&1 | tee ${OUT_FILE}
cat ${OUT_FILE} | go-junit-report > build/junit-apm-server-report.xml
for i in "full.cov" "integration.cov" "system.cov" "unit.cov"
do
  name=$(basename ${i} .cov)
  if [ -f "${COV_DIR}/${i}" ]; then 
    go tool cover -html="${COV_DIR}/${i}" -o build/coverage-${name}-report.html
    gocover-cobertura < "${COV_DIR}/${i}" > build/coverage-${name}-report.xml
  fi
done
exit 0
