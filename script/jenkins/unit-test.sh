#!/usr/bin/env bash
set -exuo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

export OUT_FILE="build/test-report.out"

make update prepare-tests test-deps
(go test -race ./... -v 2>&1 | tee ${OUT_FILE}) || echo -e "\033[31;49mTests FAILED\033[0m"
cat ${OUT_FILE} | go-junit-report > build/junit-apm-server-report.xml
