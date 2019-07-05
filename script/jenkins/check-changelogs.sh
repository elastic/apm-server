#!/usr/bin/env bash
set -exuo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

## Instal CI dependencies to run the script in python 3.6
VERSION="3.6.8"
### Install pyenv
git clone https://github.com/pyenv/pyenv.git ~/.pyenv
export PYENV_ROOT="$HOME/.pyenv"
export PATH="$PYENV_ROOT/bin:$PATH"

### Install pyenv-virtualenv plugin
git clone https://github.com/pyenv/pyenv-virtualenv.git $(pyenv root)/plugins/pyenv-virtualenv
eval "$(pyenv virtualenv-init -)"

### Install the python ersion and setup environment
pyenv install ${VERSION}
pyenv virtualenv ${VERSION} my-virtual-env-${VERSION}
pip install requests

## Run the goal
make check-changelogs
