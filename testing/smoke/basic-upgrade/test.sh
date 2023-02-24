#!/bin/bash

set -eo pipefail

# Load common lib
. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

# Get all the versions from the current region.
get_versions

VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
    VERSION=$(echo ${VERSIONS} | jq -r 'last')
    echo "-> unspecified version, using $(echo ${VERSION} | cut -d '.' -f1-2)"
fi
MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )
MINOR_VERSION=$(echo ${VERSION} | cut -d '.' -f2 )

if [[ ${MAJOR_VERSION} -eq 7 ]]; then
    ASSERT_EVENTS_FUNC=legacy_assertions
    INTEGRATIONS_SERVER=false
    get_latest_patch "${MAJOR_VERSION}.${MINOR_VERSION}"
    LATEST_VERSION=${MAJOR_VERSION}.${MINOR_VERSION}.${LATEST_PATCH}
    PREV_LATEST_VERSION=$(echo ${MAJOR_VERSION}.${MINOR_VERSION}.$(( ${LATEST_PATCH} -1 )))
elif [[ ${MAJOR_VERSION} -eq 8 ]]; then
    ASSERT_EVENTS_FUNC=data_stream_assertions
    INTEGRATIONS_SERVER=true

    get_latest_patch "${MAJOR_VERSION}.${MINOR_VERSION}"
    LATEST_VERSION=${MAJOR_VERSION}.${MINOR_VERSION}.${LATEST_PATCH}

    PREV_MINOR=$(( ${MINOR_VERSION} -1 ))
    get_latest_patch "${MAJOR_VERSION}.${PREV_MINOR}"
    PREV_LATEST_VERSION=${MAJOR_VERSION}.${PREV_MINOR}.${LATEST_PATCH}
else
    echo "version ${VERSION} not supported"
    exit 5
fi

# Check if we are testing upgrade over the same version
if [ "${LATEST_VERSION}" == "${PREV_LATEST_VERSION}" ]; then
    echo "Latest version '${LATEST_VERSION}' and previous latest version '${PREV_LATEST_VERSION}' must be different"
    exit 5
fi

echo "-> Running basic upgrade smoke test for version ${VERSION}"

if [[ -z ${SKIP_DESTROY} ]]; then
    trap "terraform_destroy" EXIT
fi

cleanup_tfvar
append_tfvar "stack_version" ${PREV_LATEST_VERSION}
append_tfvar "integrations_server" ${INTEGRATIONS_SERVER}
terraform_apply
healthcheck 1
send_events
${ASSERT_EVENTS_FUNC} ${STACK_VERSION}

echo "-> Upgrading APM Server from ${STACK_VERSION} to ${LATEST_VERSION}"
cleanup_tfvar
append_tfvar "stack_version" ${LATEST_VERSION}
append_tfvar "integrations_server" ${INTEGRATIONS_SERVER}
terraform_apply
healthcheck 1
send_events
${ASSERT_EVENTS_FUNC} ${STACK_VERSION}
