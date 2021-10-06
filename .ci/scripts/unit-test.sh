#!/usr/bin/env bash
set -exuo pipefail

source ./script/common.bash

jenkins_setup

export OUT_FILE="build/test-report.out"
export COV_DIR="build/coverage"

mkdir -p ${COV_DIR}

make update
mage goTestUnit
