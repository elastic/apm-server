#!/usr/bin/env bash
set -euox pipefail

# shellcheck disable=SC1091
source ./script/common.bash

jenkins_setup

./script/jenkins/build.sh
