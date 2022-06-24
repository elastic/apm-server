#!/bin/bash

set -eo pipefail

VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
    VERSIONS=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions)
    VERSION=$(echo ${VERSIONS} | jq -r '.versions[]' | grep -v 'SNAPSHOT' | tail -1)
    echo "-> unspecified version, using $(echo ${VERSION} | cut -d '.' -f1-2)"
fi
MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )
MINOR_VERSION=$(echo ${VERSION} | cut -d '.' -f2 )

if [[ ${MAJOR_VERSION} -eq 7 ]]; then
    ASSERT_EVENTS_FUNC=legacy_assert_events
    LATEST_VERSION=$(curl -s --fail https://artifacts-api.elastic.co/v1/versions/${MAJOR_VERSION}.${MINOR_VERSION} | jq -r '.version.builds[0].version')
    PREV_LATEST_VERSION=$(echo ${MAJOR_VERSION}.${MINOR_VERSION}.$(( $(echo ${LATEST_VERSION} | cut -d '.' -f3) -1 )))
elif [[ ${MAJOR_VERSION} -eq 8 ]]; then
    ASSERT_EVENTS_FUNC=data_stream_assert_events
    LATEST_VERSION=$(echo ${VERSIONS} | jq -r '.versions[]' | grep -v 'SNAPSHOT' | grep ${VERSION} | tail -1)
    PREV_LATEST_VERSION=$(echo ${VERSIONS} | jq -r '.versions[]' | grep -v 'SNAPSHOT' | grep $(echo ${MAJOR}.$(( $(echo ${MINOR_VERSION} | cut -d '.' -f3) -1 ))) | tail -1)
    INTEGRATIONS_SERVER=true
else
    echo "version ${VERSION} not supported"
    exit 5
fi

. $(git rev-parse --show-toplevel)/testing/smoke/lib.sh

trap "terraform_destroy" EXIT

terraform_apply ${PREV_LATEST_VERSION} ${INTEGRATIONS_SERVER}

ELASTICSEARCH_URL=$(terraform output -raw elasticsearch_url)
ELASTICSEARCH_USER=$(terraform output -raw elasticsearch_username)
ELASTICSEARCH_PASS=$(terraform output -raw elasticsearch_password)
APM_AUTH_HEADER="Authorization: Bearer $(terraform output -raw apm_secret_token)"
APM_SERVER_URL=$(terraform output -raw apm_server_url)

healthcheck 1
send_events
${ASSERT_EVENTS_FUNC} ${PREV_LATEST_VERSION}

echo "-> Upgrading APM Server to ${LATEST_VERSION}"
echo stack_version=\"${LATEST_VERSION}\" > terraform.tfvars
terraform_apply ${LATEST_VERSION} ${INTEGRATIONS_SERVER}

healthcheck 1
send_events
${ASSERT_EVENTS_FUNC} ${LATEST_VERSION}

echo "-> Smoke tests passed!"
