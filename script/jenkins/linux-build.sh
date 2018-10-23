#!/usr/bin/env bash
set -euox pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
  make stop-environment fix-permissions
}
trap cleanup EXIT

make clean
make
