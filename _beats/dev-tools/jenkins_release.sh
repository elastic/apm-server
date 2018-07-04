#!/usr/bin/env bash
set -euox pipefail

source ./_beats/dev-tools/common.bash && jenkins_setup

cleanup() {
  rm -rf $TEMP_PYTHON_ENV
}
trap cleanup EXIT

# Run the deploy script with the appropriate version in the environment.
cd ${WORKSPACE}/src/github.com/elastic/apm-server
make SNAPSHOT=yes clean update package

ln -s ${WORKSPACE}/src/github.com/elastic/apm-server/build/upload ${WORKSPACE}/src/github.com/elastic/apm-server/build/distributions
