#!/usr/bin/env bash
set -euox pipefail

: "${HOME:?Need to set HOME to a non-empty value.}"
: "${WORKSPACE:?Need to set WORKSPACE to a non-empty value.}"

# Setup Go.
export GOPATH=${WORKSPACE}
export PATH=${GOPATH}/bin:${PATH}
go_file="_beats/.go-version"
if [ -f "$go_file" ]; then
  eval "$(gvm $(cat $go_file))"
else
  eval "$(gvm 1.9.4)"
fi

# Workaround for Python virtualenv path being too long.
TEMP_PYTHON_ENV=$(mktemp -d)
export PYTHON_ENV="${TEMP_PYTHON_ENV}/python-env"

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
}
trap cleanup EXIT

make check-full
