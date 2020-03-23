#!/usr/bin/env bash
set -euox pipefail

# Setup python3
curl -L https://github.com/pyenv/pyenv-installer/raw/master/bin/pyenv-installer | bash
pyenv install 3.7.7

# shellcheck disable=SC1091
source ./script/common.bash

jenkins_setup

script/jenkins/unit-test.sh
