#!/usr/bin/env bash
set -euox pipefail

source ./script/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
}
trap cleanup EXIT

make are-kibana-objects-updated
