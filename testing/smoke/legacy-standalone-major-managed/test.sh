#!/bin/bash

set -eo pipefail

if [[ "${1}" != "7.17" && "${1}" != "latest" ]]; then
    echo "-> Skipping smoke test ['${1}' is not supported]..."
    exit 0
fi

. "$(git rev-parse --show-toplevel)/testing/smoke/lib.sh"

VERSION_7=7.17
if [[ "${1}" == "latest" ]]; then
    # SNAPSHOT version can only be upgraded to another SNAPSHOT version
    get_latest_snapshot
    LATEST_VERSION_7=$(echo "${VERSIONS}" | jq -r -c "map(select(. | startswith(\"${VERSION_7}\"))) | .[-1]")
    ASSERTION_VERSION_7=${LATEST_VERSION_7%-*} # strip -SNAPSHOT suffix
    LATEST_VERSION_8=$(echo "${VERSIONS}" | jq -r '[.[] | select(. | startswith("8"))] | last')
    ASSERTION_VERSION_8=${LATEST_VERSION_8%-*} # strip -SNAPSHOT suffix
else
    get_versions
    LATEST_VERSION_7=$(echo "${VERSIONS}" | jq -r -c "map(select(. | startswith(\"${VERSION_7}\"))) | .[-1]")
    ASSERTION_VERSION_7=${LATEST_VERSION_7}
    LATEST_VERSION_8=$(echo "${VERSIONS}" | jq -r '[.[] | select(. | startswith("8"))] | last')
    ASSERTION_VERSION_8=${LATEST_VERSION_8}
fi

echo "-> Running ${LATEST_VERSION_7} standalone to ${LATEST_VERSION_8} standalone to ${LATEST_VERSION_8} managed"

if [[ -z ${SKIP_DESTROY} ]]; then
    trap "terraform_destroy" EXIT
fi

# Version 7
INTEGRATIONS_SERVER=false
cleanup_tfvar
append_tfvar "stack_version" "${LATEST_VERSION_7}"
append_tfvar "integrations_server" ${INTEGRATIONS_SERVER}
terraform_apply
healthcheck 1
send_events
legacy_assertions "${ASSERTION_VERSION_7}"

# Version 8
cleanup_tfvar
append_tfvar "stack_version" "${LATEST_VERSION_8}"
append_tfvar "integrations_server" ${INTEGRATIONS_SERVER}
terraform_apply
healthcheck 1
send_events
data_stream_assertions "${ASSERTION_VERSION_8}"

MANAGED_VERSION="${LATEST_VERSION_8}"
ASSERTION_MANAGED_VERSION="${ASSERTION_VERSION_8}"

upgrade_managed "${MANAGED_VERSION}"
healthcheck 1
send_events
# Assert there are 2 instances of the same event, since we ingested data twice
# using the same APM Server version.
data_stream_assertions "${ASSERTION_MANAGED_VERSION}" 2
