#!/usr/bin/env bash
set -e

source ./_beats/dev-tools/common.bash

jenkins_setup

go get -u golang.org/x/tools/cmd/benchcmp
make bench > bench.out
