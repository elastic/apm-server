#!/bin/bash

set -eo pipefail

# Found current script directory
readonly RELATIVE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Extract version from arguments
VERSION=${1}
if [ -z "${VERSION}" ]; then
    echo "Please provide a version to use for tests."
    exit 1
fi

${RELATIVE_DIR}/basic-upgrade.sh "${VERSION}"

if [ "${VERSION}" == "7.17" ]; then
    ${RELATIVE_DIR}/legacy-managed.sh && ${RELATIVE_DIR}/standalone-major-managed.sh
fi
