#!/usr/bin/env bash
set -euxo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

make test-deps
make bench | tee bench.out
