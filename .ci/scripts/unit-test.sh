#!/usr/bin/env bash
set -exuo pipefail

source ./script/common.bash

jenkins_setup

export OUT_FILE="build/TEST-go-unit"

go install -modfile=tools/go.mod github.com/t-yuki/gocover-cobertura

mkdir -p ${COV_DIR}

make update
mage goTestUnit
gocover-cobertura < "${OUT_FILE}.cov" > "${OUT_FILE}.xml"
