#!/usr/bin/env bash
set -exuo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

## Instal CI dependencies to run the script in python 3.6
git clone https://github.com/pyenv/pyenv-virtualenv.git $(pyenv root)/plugins/pyenv-virtualenv
eval "$(pyenv virtualenv-init -)"
pyenv install 3.6.8
pyenv virtualenv 3.6.8 my-virtual-env-3.6.8
pip install requests

## Run the goal
make check-changelogs
