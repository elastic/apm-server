#!/bin/bash

set -eo pipefail

export TF_IN_AUTOMATION=1
export TF_CLI_ARGS=-no-color

# Found current script directory
readonly RELATIVE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
readonly SCRIPT_NAME=$(basename $(pwd))

# Extract version from arguments
VERSION=${1}
if [ -z "${VERSION}" ]; then
    echo "Please provide a version to use for tests."
    exit 1
fi

${RELATIVE_DIR}/${SCRIPT_NAME}.sh "${VERSION}"
