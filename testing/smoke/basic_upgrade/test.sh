#!/bin/bash

set -eo pipefail

# Found current script directory
readonly RELATIVE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Load common lib
. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh
VERSION=${1}

${RELATIVE_DIR}/basic_upgrade.sh "${VERSION}"

if [ "${VERSION}" == "7.17" ]; then
    ${RELATIVE_DIR}/legacy-managed.sh && ${RELATIVE_DIR}/standalone-major-managed.sh
fi
