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

echo "-> Running ${LATEST_VERSION} standalone to ${LATEST_VERSION} managed upgrade"

if [[ -z ${SKIP_DESTROY} ]]; then
    trap "terraform_destroy" EXIT
fi

INTEGRATIONS_SERVER=false
cleanup_tfvar
append_tfvar "stack_version" ${LATEST_VERSION}
append_tfvar "integrations_server" ${INTEGRATIONS_SERVER}
terraform_apply
healthcheck 1
send_events
legacy_assertions ${LATEST_VERSION}

echo "-> Upgrading APM Server to managed mode"
upgrade_managed ${LATEST_VERSION} ${INTEGRATIONS_SERVER}
healthcheck 1
send_events
data_stream_assertions ${LATEST_VERSION}
