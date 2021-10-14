#!/usr/bin/env bash
set -eox pipefail

# Setup python3
pythonVersion=3.7.7
rm -fr "${HOME}/.pyenv"
curl -L https://github.com/pyenv/pyenv-installer/raw/master/bin/pyenv-installer | bash
export PATH="${HOME}/.pyenv/bin:$PATH"
eval "$(pyenv init -)"
eval "$(pyenv virtualenv-init -)"
pyenv install ${pythonVersion}
pyenv global ${pythonVersion}
pythonInstallation=$(dirname "$(pyenv which pip)")
export PATH="${pythonInstallation}:$PATH"

# shellcheck disable=SC1091
source ./script/common.bash

jenkins_setup

./.ci/scripts/unit-test.sh
