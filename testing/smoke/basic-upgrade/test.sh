#!/bin/bash

set -eo pipefail

# Load common lib
. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

ARTIFACTS_API=https://artifacts-api.elastic.co/v1
# Load the latest versions except SNAPSHOTS
VERSIONS=$(curl -s --fail $ARTIFACTS_API/versions | jq -r -c '[.versions[] | select(. | endswith("-SNAPSHOT") | not)] | sort')

VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
    VERSION=$(echo ${VERSIONS} | jq -r 'last')
    echo "-> unspecified version, using $(echo ${VERSION} | cut -d '.' -f1-2)"
fi
MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )
MINOR_VERSION=$(echo ${VERSION} | cut -d '.' -f2 )

if [[ ${MAJOR_VERSION} -eq 7 ]]; then
    ASSERT_EVENTS_FUNC=legacy_assertions
    # Check if the version is available.
    if ! curl -s --fail $ARTIFACTS_API/versions/${MAJOR_VERSION}.${MINOR_VERSION} ; then
        echo "-> Skipping there are no artifacts to be downloaded in artifacts-api.elastic.co ..."
        exit 0
    fi
    LATEST_VERSION=$(curl -s --fail $ARTIFACTS_API/versions/${MAJOR_VERSION}.${MINOR_VERSION} | jq -r '.version.builds[0].version')
    PREV_LATEST_VERSION=$(echo ${MAJOR_VERSION}.${MINOR_VERSION}.$(( $(echo ${LATEST_VERSION} | cut -d '.' -f3) -1 )))
elif [[ ${MAJOR_VERSION} -eq 8 ]]; then
    ASSERT_EVENTS_FUNC=data_stream_assertions
    LATEST_VERSION=$(echo ${VERSIONS} | jq -r "[.[] | select(. | contains(\"${VERSION}\"))] | last")
    # $ARTIFACTS_API/versions only provides the last two
    # major versions and minor versions. For that reason, we use a regex.
    PREV_LATEST_VERSION=$(echo "${MAJOR_VERSION}.$(( ${MINOR_VERSION} -1 )).[0-9]?([0-9])\$")
    INTEGRATIONS_SERVER=true
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

terraform_apply ${PREV_LATEST_VERSION} ${INTEGRATIONS_SERVER}
healthcheck 1
send_events
${ASSERT_EVENTS_FUNC} ${STACK_VERSION}

echo "-> Upgrading APM Server from ${STACK_VERSION} to ${LATEST_VERSION}"
terraform_apply ${LATEST_VERSION} ${INTEGRATIONS_SERVER}
healthcheck 1
send_events
${ASSERT_EVENTS_FUNC} ${STACK_VERSION}
