#!/bin/bash

set -eo pipefail

if [[ ${1} != 7.17 ]]; then
    echo "-> Skipping smoke test..."
    exit 0
fi

VERSION=7.17
# Check if the version is available.
if ! curl --fail https://artifacts-api.elastic.co/v1/versions/${VERSION} ; then
    echo "-> Warning there are no new artifacts to be downloaded in artifacts-api.elastic.co ..."
    exit 0
fi

LATEST_VERSION=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions/${VERSION} | jq -r '.version.builds[0].version')

echo "-> Running ${LATEST_VERSION} standalone to ${LATEST_VERSION} managed upgrade"

. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

trap "terraform_destroy" EXIT

terraform_apply ${LATEST_VERSION}
healthcheck 1
send_events
legacy_assertions ${LATEST_VERSION}

echo "-> Upgrading APM Server to managed mode"
upgrade_managed ${LATEST_VERSION}
healthcheck 1
send_events
data_stream_assertions ${LATEST_VERSION}
