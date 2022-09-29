#!/bin/bash

set -eo pipefail

if [[ "${1}" != "7.17" ]]; then
    echo "-> Skipping smoke test ['${1}' is not supported]..."
    exit 0
fi

. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

VERSION=7.17
get_versions
get_latest_patch ${VERSION}
LATEST_VERSION=${VERSION}.${LATEST_PATCH}
NEXT_MAJOR_LATEST=$(echo ${VERSIONS} | jq -r '[.[] | select(. | startswith("8"))] | last')

echo "-> Running ${LATEST_VERSION} standalone to ${NEXT_MAJOR_LATEST} to ${NEXT_MAJOR_LATEST} managed"

if [[ -z ${SKIP_DESTROY} ]]; then
    trap "terraform_destroy" EXIT
fi

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
# Assert there are 2 instances of the same event, since we ingested data twice
# using the same APM Server version.
data_stream_assertions ${NEXT_MAJOR_LATEST} 2
