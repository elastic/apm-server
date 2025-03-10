#!/usr/bin/env bash

set -eo pipefail

KEY_NAME="provisioner_key_$(date +%s)"

ssh-keygen -f ${KEY_NAME} -N ""

# Load common lib
. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

# Get all the snapshot versions from the current region.
get_latest_snapshot

<<<<<<< HEAD
VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
    # NOTE(marclop) Temporarily avoid testing against 9.x, since we want to test that the
    # upgrade for 7.17 to 8.latest works correctly.
    # Uncomment the line below when we are ready to test against 9.x and delete the line
    # after the next one.
    # VERSION=$(echo ${VERSIONS} | jq -r 'last')
    VERSION=$(echo ${VERSIONS} | jq -r '[.[] | select(. | startswith("8"))] | last')
    echo "-> unspecified version, using $(echo ${VERSION} | cut -d '.' -f1-2)"
fi
MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )
MINOR_VERSION=$(echo ${VERSION} | cut -d '.' -f2 )

=======
# APM `major.minor` version e.g. 8.17.
APM_SERVER_VERSION=$(echo ${1} | cut -d '.' -f1-2)
# `VERSIONS` only contains snapshot versions and is in sorted order.
# We retrieve the appropriate stack snapshot version from the list by:
# 1. Selecting the ones that start with APM's `major.minor`.
# 2. Get the last one, which should be latest.
VERSION=$(echo ${VERSIONS} | jq -r --arg VS ${APM_SERVER_VERSION} '[.[] | select(. | startswith($VS))] | last')
>>>>>>> 5ef0045f (smoke: Use APM Server version to get Stack version (#16035))
OBSERVER_VERSION=$(echo ${VERSION} | cut -d '-' -f1 )
MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )

if [[ ${MAJOR_VERSION} -eq 8 ]]; then
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
