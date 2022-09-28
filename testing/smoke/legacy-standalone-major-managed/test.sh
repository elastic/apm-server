#!/bin/bash

set -eo pipefail

if [[ ${1} != 7.17 ]]; then
    echo "-> Skipping smoke test..."
    exit 0
fi

VERSION=7.17

# Load all versions except SNAPSHOTS
VERSIONS=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions | jq -r -c '[.versions[] | select(. | endswith("-SNAPSHOT") | not)] | sort')
NEXT_MAJOR_LATEST=$(echo ${VERSIONS} | jq -r '[.[] | select(. | startswith("8"))] | last')
# Check if the version is available.
if ! curl --fail https://artifacts-api.elastic.co/v1/versions/${VERSION} ; then
    echo "-> Warning there are no new artifacts to be downloaded in artifacts-api.elastic.co ..."
    exit 0
fi
LATEST_VERSION=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions/${VERSION} | jq -r '.version.builds[0].version')

echo "-> Running ${LATEST_VERSION} standalone to ${NEXT_MAJOR_LATEST} to ${NEXT_MAJOR_LATEST} managed"

. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

trap "terraform_destroy" EXIT

terraform_apply ${LATEST_VERSION}
healthcheck 1
send_events
legacy_assertions ${LATEST_VERSION}

terraform_apply ${NEXT_MAJOR_LATEST}
healthcheck 1
send_events
data_stream_assertions ${NEXT_MAJOR_LATEST}

upgrade_managed ${NEXT_MAJOR_LATEST}
healthcheck 1
send_events
# Assert there are 2 instances of the same event, since we ingested data twice.
data_stream_assertions ${NEXT_MAJOR_LATEST} 2
