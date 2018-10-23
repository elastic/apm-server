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

#export GOPACKAGES=$(go list github.com/elastic/apm-server/...| grep -v /vendor/ | grep -v /scripts/cmd/)

go get -v -u github.com/jstemmer/go-junit-report

go get -v -u github.com/axw/gocov/gocov
go get -v -u gopkg.in/matm/v1/gocov-html

go get -v -u github.com/axw/gocov/...
go get -v -u github.com/AlekSi/gocov-xml

export COV_DIR="build/coverage"
export OUT_FILE="build/test-report.out"
mkdir -p build
#go test -race ${GOPACKAGES} -v 2>&1 | tee ${OUT_FILE}
go test -race ./... -v 2>&1 | tee ${OUT_FILE}
cat ${OUT_FILE} | go-junit-report > build/junit-apm-server-report.xml
for i in "full.cov" "integration.cov" "system.cov" "unit.cov"
do
  name=$(basename ${i} .cov)
  [ -f "${COV_DIR}/${i}" ] && gocov convert "${COV_DIR}/${i}" | gocov-html > build/coverage-${name}-report.html
  [ -f "${COV_DIR}/${i}" ] && gocov convert "${COV_DIR}/${i}" | gocov-xml > build/coverage-${name}-report.xml
done
exit 0
