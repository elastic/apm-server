#!/usr/bin/env bash
set -euo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
}
trap cleanup EXIT

make clean
make
