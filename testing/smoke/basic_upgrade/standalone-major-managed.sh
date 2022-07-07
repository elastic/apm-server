#!/bin/bash

set -eo pipefail

VERSION=7.17
LATEST_VERSION=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions/${VERSION} | jq -r '.version.builds[0].version')
VERSIONS=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions)
NEXT_MAJOR_LATEST=$(echo ${VERSIONS} | jq -r '.versions[]' | grep -v 'SNAPSHOT' | grep '^8' | tail -1)

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
data_stream_assert_events ${NEXT_MAJOR_LATEST}

upgrade_managed ${NEXT_MAJOR_LATEST}
healthcheck 1
send_events
# Assert there are 2 instances of the same event, since we ingested data twice.
data_stream_assert_events ${NEXT_MAJOR_LATEST} 2
