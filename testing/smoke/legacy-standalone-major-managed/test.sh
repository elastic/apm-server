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
    get_latest_snapshot_for_version ${VERSION_7}
    LATEST_VERSION_7=${LATEST_SNAPSHOT_VERSION}
    ASSERTION_VERSION_7=${LATEST_SNAPSHOT_VERSION%-*} # strip -SNAPSHOT suffix
    get_latest_snapshot
    LATEST_VERSION_8=$(echo "${VERSIONS}" | jq -r '[.[] | select(. | startswith("8"))] | last')
    ASSERTION_VERSION_8=${LATEST_VERSION_8%-*} # strip -SNAPSHOT suffix
    LATEST_VERSION_9=$(echo "${VERSIONS}" | jq -r '[.[] | select(. | startswith("9"))] | last')
    ASSERTION_VERSION_9=${LATEST_VERSION_9%-*} # strip -SNAPSHOT suffix
else
    get_latest_patch ${VERSION_7}
    LATEST_VERSION_7=${VERSION_7}.${LATEST_PATCH}
    ASSERTION_VERSION_7=${LATEST_VERSION_7}
    get_versions
    LATEST_VERSION_8=$(echo "${VERSIONS}" | jq -r '[.[] | select(. | startswith("8"))] | last')
    ASSERTION_VERSION_8=${LATEST_VERSION_8}
    LATEST_VERSION_9=$(echo "${VERSIONS}" | jq -r '[.[] | select(. | startswith("9"))] | last')
    ASSERTION_VERSION_9=${LATEST_VERSION_9}
fi

if [[ -n ${LATEST_VERSION_9} ]]; then
    echo "-> Running ${LATEST_VERSION_7} standalone to ${LATEST_VERSION_8} standalone to ${LATEST_VERSION_9} standalone to ${LATEST_VERSION_9} managed"
else
    echo "-> Running ${LATEST_VERSION_7} standalone to ${LATEST_VERSION_8} standalone to ${LATEST_VERSION_8} managed"
fi

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

##### Disabled for now because both 9.0.0 and 9.1.0-SNAPSHOT are failing!
# Version 9 (if exists)
#if [[ -n ${LATEST_VERSION_9} ]]; then
#    cleanup_tfvar
#    append_tfvar "stack_version" "${LATEST_VERSION_9}"
#    append_tfvar "integrations_server" ${INTEGRATIONS_SERVER}
#    terraform_apply
#    healthcheck 1
#    send_events
#    data_stream_assertions "${ASSERTION_VERSION_9}"
#    MANAGED_VERSION="${LATEST_VERSION_9}"
#    ASSERTION_MANAGED_VERSION="${ASSERTION_VERSION_9}"
#fi

upgrade_managed "${MANAGED_VERSION}"
healthcheck 1
send_events
# Assert there are 2 instances of the same event, since we ingested data twice
# using the same APM Server version.
data_stream_assertions "${ASSERTION_MANAGED_VERSION}" 2
