#!/usr/bin/env bash
set -eox pipefail

pythonVersion=3.10.0

if command -v python 2> /dev/null ; then
    set +e
    echo "Found Python. Checking version.."
    FOUND_PYTHON_VERSION=$(python --version|awk '{print $2}')
    if [ "$FOUND_PYTHON_VERSION" == "$pythonVersion" ]
    then
        echo "Versions match. No need to install Python. Exiting."
        exit 0
    fi
    set -e
fi

# Setup python3
rm -fr "${HOME}/.pyenv"
if ! command -v pyenv 2> /dev/null ; then
    curl -L https://github.com/pyenv/pyenv-installer/raw/master/bin/pyenv-installer | bash
    export PATH="${HOME}/.pyenv/bin:$PATH"
fi
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
