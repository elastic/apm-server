#!/usr/bin/env bash

set -eo pipefail

KEY_NAME="provisioner_key_$(date +%s)"

ssh-keygen -f ${KEY_NAME} -N ""

# Load common lib
. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

# Get all the versions from the current region.
get_latest_snapshot

VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
    VERSION=$(echo ${VERSIONS} | jq -r 'last')
    echo "-> unspecified version, using $(echo ${VERSION} | cut -d '.' -f1-2)"
fi
MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )
MINOR_VERSION=$(echo ${VERSION} | cut -d '.' -f2 )

OBSERVER_VERSION=$(echo ${VERSION} | cut -d '-' -f1 )

if [[ ${MAJOR_VERSION} -eq 8 ]]; then
    ASSERT_EVENTS_FUNC=data_stream_assertions
    INTEGRATIONS_SERVER=true

    get_latest_patch "${MAJOR_VERSION}.${MINOR_VERSION}"
    LATEST_VERSION=${MAJOR_VERSION}.${MINOR_VERSION}.${LATEST_PATCH}
else
    echo "version ${VERSION} not supported"
    exit 5
fi

echo "-> Running basic supported OS smoke test for version ${VERSION}"

if [[ -z ${SKIP_DESTROY} ]]; then
    trap "terraform_destroy 0" EXIT
fi

. $(git rev-parse --show-toplevel)/testing/smoke/os_matrix.sh

for os in "${os_names[@]}"
do
    cleanup_tfvar
    append_tfvar "aws_provisioner_key_name" ${KEY_NAME}
    append_tfvar "aws_os" $os
    append_tfvar "stack_version" ${VERSION}
    terraform_apply
    # The previous test case's APM Server should have been stopped by now,
    # so there should be no new documents indexed. Delete all existing data.
    delete_all
    healthcheck 1
    send_events
    ${ASSERT_EVENTS_FUNC} ${OBSERVER_VERSION}
done
