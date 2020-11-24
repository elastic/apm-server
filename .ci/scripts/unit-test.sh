#!/usr/bin/env bash
set -exuo pipefail

source ./script/common.bash

jenkins_setup

export OUT_FILE="build/test-report.out"
export COV_DIR="build/coverage"

mkdir -p ${COV_DIR}

make update
go install github.com/jstemmer/go-junit-report
go install github.com/t-yuki/gocover-cobertura

(go test -race -covermode=atomic -coverprofile=${COV_DIR}/unit.cov -v ./... 2>&1 | tee ${OUT_FILE}) || echo -e "\033[31;49mTests FAILED\033[0m"

cat ${OUT_FILE} | go-junit-report > build/junit-apm-server-report.xml
go tool cover -html="${COV_DIR}/unit.cov" -o "${COV_DIR}/coverage-unit-report.html"
gocover-cobertura < "${COV_DIR}/unit.cov" > "${COV_DIR}/coverage-unit-report.xml"
