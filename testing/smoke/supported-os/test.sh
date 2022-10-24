#!/bin/bash

set -eo pipefail

KEY_NAME="provisioner_key"

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

if [[ ${MAJOR_VERSION} -eq 7 ]]; then
    ASSERT_EVENTS_FUNC=legacy_assertions
    INTEGRATIONS_SERVER=false
    get_latest_patch "${MAJOR_VERSION}.${MINOR_VERSION}"
    LATEST_VERSION=${MAJOR_VERSION}.${MINOR_VERSION}.${LATEST_PATCH}
elif [[ ${MAJOR_VERSION} -eq 8 ]]; then
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
    trap "terraform_destroy" EXIT
fi

os_names=(
    "ubuntu-bionic-18.04-amd64-server"
    "ubuntu-focal-20.04-amd64-server"
    "ubuntu-jammy-22.04-amd64-server"
    "debian-10-amd64"
    "debian-11-amd64"
    "amzn2-ami-kernel-5.10"
    "RHEL-7"
    "RHEL-8"
)

for os in "${os_names[@]}"
do
    terraform_apply ${LATEST_VERSION} ${INTEGRATIONS_SERVER} ${KEY_NAME} $os
    healthcheck 1
    send_events
    ${ASSERT_EVENTS_FUNC} ${OBSERVER_VERSION}
    terraform_destroy
done
