#!/usr/bin/env bash

set -eo pipefail

KEY_NAME="provisioner_key_$(date +%s)"

ssh-keygen -f ${KEY_NAME} -N ""

# Load common lib
. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

# Get all the snapshot versions from the current region.
get_latest_snapshot

# APM `major.minor` version e.g. 8.17.
APM_SERVER_VERSION=$(echo ${1} | cut -d '.' -f1-2)
# `VERSIONS` only contains snapshot versions and is in sorted order.
# We retrieve the appropriate stack snapshot version from the list by:
# 1. Selecting the ones that start with APM's `major.minor`.
# 2. Get the last one, which should be latest.
VERSION=$(echo ${VERSIONS} | jq -r --arg VS ${APM_SERVER_VERSION} '[.[] | select(. | startswith($VS))] | last')
OBSERVER_VERSION=$(echo ${VERSION} | cut -d '-' -f1 )
MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )

if [[ ${MAJOR_VERSION} -eq 8 ]] || [[ ${MAJOR_VERSION} -eq 9 ]]; then
    ASSERT_EVENTS_FUNC=data_stream_assertions
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
    append_tfvar "aws_os" "$os"
    append_tfvar "stack_version" ${VERSION}
    terraform_apply
    # The previous test case's APM Server should have been stopped by now,
    # so there should be no new documents indexed. Delete all existing data.
    delete_all
    healthcheck 1
    inspect_systemd_ulimits || true
    send_events
    ${ASSERT_EVENTS_FUNC} ${OBSERVER_VERSION}
done
