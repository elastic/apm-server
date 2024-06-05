#!/bin/bash

set -eo pipefail

if [[ "${1}" != "7.17" && "${1}" != "latest" ]]; then
    echo "-> Skipping smoke test ['${1}' is not supported]..."
    exit 0
fi

. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

VERSION=7.17
get_versions
if [[ "${1}" == "latest" ]]; then
    # a SNAPSHOT version can only be upgraded to another SNAPSHOT version
    get_latest_snapshot_for_version ${VERSION}
    LATEST_VERSION=${LATEST_SNAPSHOT_VERSION}
    ASSERTION_VERSION=${LATEST_SNAPSHOT_VERSION%-*} # strip -SNAPSHOT suffix
    get_latest_snapshot
    NEXT_MAJOR_LATEST=$(echo $VERSIONS | jq -r -c '.[-1]')
    ASSERTION_NEXT_MAJOR_LATEST=${NEXT_MAJOR_LATEST%-*} # strip -SNAPSHOT suffix
else
    get_latest_patch ${VERSION}
    LATEST_VERSION=${VERSION}.${LATEST_PATCH}
    ASSERTION_VERSION=${LATEST_VERSION}
    NEXT_MAJOR_LATEST=$(echo ${VERSIONS} | jq -r '[.[] | select(. | startswith("8"))] | last')
    ASSERTION_NEXT_MAJOR_LATEST=${NEXT_MAJOR_LATEST}
fi

# temporary fix to use SNAPSHOT image as 7.17.22 is broken
if [ $LATEST_VERSION == "7.17.22" ]; then 
    echo "using 7.17.22-SNAPSHOT instead of 7.17.22 temporarily"
    LATEST_VERSION="${LATEST_VERSION}-SNAPSHOT"
fi

# temporary fix to use SNAPSHOT image as 7.17.22 is broken
if [ $NEXT_MAJOR_LATEST == "7.17.22" ]; then 
    echo "using 7.17.22-SNAPSHOT instead of 7.17.22 temporarily"
    NEXT_MAJOR_LATEST="${NEXT_MAJOR_LATEST}-SNAPSHOT"
fi


echo "-> Running ${LATEST_VERSION} standalone to ${NEXT_MAJOR_LATEST} to ${NEXT_MAJOR_LATEST} managed"

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
legacy_assertions ${ASSERTION_VERSION}

cleanup_tfvar
append_tfvar "stack_version" ${NEXT_MAJOR_LATEST}
append_tfvar "integrations_server" ${INTEGRATIONS_SERVER}
terraform_apply
healthcheck 1
send_events
data_stream_assertions ${ASSERTION_NEXT_MAJOR_LATEST}

upgrade_managed ${NEXT_MAJOR_LATEST}
healthcheck 1
send_events
# Assert there are 2 instances of the same event, since we ingested data twice
# using the same APM Server version.
data_stream_assertions ${ASSERTION_NEXT_MAJOR_LATEST} 2
