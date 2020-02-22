#!/usr/bin/env bash
set -exuo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

export OUT_FILE="build/test-report.out"
export COV_DIR="build/coverage"

make update test-deps

mkdir -p ${COV_DIR}
(go test -race -covermode=atomic -coverprofile=${COV_DIR}/unit.cov -v ./... 2>&1 | tee ${OUT_FILE}) || echo -e "\033[31;49mTests FAILED\033[0m"

cat ${OUT_FILE} | bin/go-junit-report > build/junit-apm-server-report.xml
go tool cover -html="${COV_DIR}/unit.cov" -o "${COV_DIR}/coverage-unit-report.html"
bin/gocover-cobertura < "${COV_DIR}/unit.cov" > "${COV_DIR}/coverage-unit-report.xml"
