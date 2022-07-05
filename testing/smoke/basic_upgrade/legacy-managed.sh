#!/bin/bash

set -eo pipefail

VERSION=7.17
LATEST_VERSION=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions/${VERSION} | jq -r '.version.builds[0].version')

echo "-> Running ${LATEST_VERSION} standalone to ${LATEST_VERSION} managed upgrade"

. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

trap "terraform_destroy" EXIT

terraform_apply ${LATEST_VERSION}
healthcheck 1
send_events
legacy_assert_events ${LATEST_VERSION}

echo "-> Upgrading APM Server to managed mode"
upgrade_managed ${LATEST_VERSION}
healthcheck 1
send_events
data_stream_assert_events ${LATEST_VERSION}
