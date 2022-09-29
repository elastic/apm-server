#!/usr/bin/env bash

set -eo pipefail

VERSION="${1}"
if [[ "${1}" != "7.17" ]]; then
    echo "-> Skipping smoke test ['${VERSION}' is not supported]..."
    exit 0
fi

. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

VERSION=7.17
get_versions
get_latest_patch ${VERSION}
LATEST_VERSION=${VERSION}.${LATEST_PATCH}

echo "-> Running ${LATEST_VERSION} standalone to ${LATEST_VERSION} managed upgrade"

if [[ -z ${SKIP_DESTROY} ]]; then
    trap "terraform_destroy" EXIT
fi

terraform_apply ${LATEST_VERSION}
healthcheck 1
send_events
legacy_assertions ${LATEST_VERSION}

echo "-> Upgrading APM Server to managed mode"
upgrade_managed ${LATEST_VERSION}
healthcheck 1
send_events
data_stream_assertions ${LATEST_VERSION}
