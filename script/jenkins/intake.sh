#!/usr/bin/env bash
set -euxo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
}
trap cleanup EXIT

go get -u golang.org/x/tools/cmd/goimports

make check-full
