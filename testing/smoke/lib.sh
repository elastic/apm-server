#!/bin/bash

terraform_apply() {
    echo "-> Creating / Upgrading deployment to version ${1}"
    echo stack_version=\"${1}\" > terraform.tfvars
    if [[ ! -z ${2} ]] && [[ ${2} ]]; then echo integrations_server=true >> terraform.tfvars; fi
    terraform init
    terraform apply -auto-approve

    if [[ ${EXPORTED_AUTH} ]]; then
        return
    fi
    ELASTICSEARCH_URL=$(terraform output -raw elasticsearch_url)
    ELASTICSEARCH_USER=$(terraform output -raw elasticsearch_username)
    ELASTICSEARCH_PASS=$(terraform output -raw elasticsearch_password)
    APM_AUTH_HEADER="Authorization: Bearer $(terraform output -raw apm_secret_token)"
    APM_SERVER_URL=$(terraform output -raw apm_server_url)
    KIBANA_URL=$(terraform output -raw kibana_url)
    EXPORTED_AUTH=true
}

terraform_destroy() {
    exit_code=$?
    if [[ ${exit_code} -gt 0 ]]; then
        echo "-> Smoke tests FAILED!!"
    fi
    echo "-> Destroying the underlying infrastructure..." 
    terraform destroy -auto-approve
    rm -f terraform.tfvars
    exit ${exit_code}
}

assert_document() {
    local INDEX=${1}
    local FIELD=${2}
    local VALUE=${3}
    local VERSION=${4}
    local ENTRIES=${5}
    if [[ -z ${ENTRIES} ]]; then ENTRIES=1; fi
    local AUTH=${ELASTICSEARCH_USER}:${ELASTICSEARCH_PASS}
    local URL=${ELASTICSEARCH_URL}/${INDEX}/_search
    # RESULT needs to be a global variable in order to be able to parse
    # the whole result in assert_entry. Passing it as a string
    # argument doesn't work well.
    RESULT=$(curl -s -u ${AUTH} -XGET "${URL}" -H 'Content-Type: application/json' -d"{\"query\":{\"bool\":{\"must\":[{\"match\":{\"${FIELD}\":\"${VALUE}\"}},{\"match\":{\"observer.version\":\"${VERSION}\"}}]}}}")

    echo "-> Asserting ${INDEX} contains expected documents documents..."
    assert_entry ${FIELD} ${VALUE} ${ENTRIES}
}

assert_entry() {
    local FIELD=${1}
    local VALUE=${2}
    local ENTRIES=${3}
    local HITS=$(echo ${RESULT} | jq .hits.total.value)
    local MSG="${FIELD}=${VALUE}"
    if [[ ${HITS} -ne ${ENTRIES} ]]; then
        echo "Didn't find ${ENTRIES} indexed documents ${MSG}, total hits ${HITS}"
        echo ${RESULT}
        exit 2
    else
        echo "-> Asserted ${ENTRIES} ${MSG} exists"
    fi
}

send_events() {
    local INTAKE_HEADER='Content-type: application/x-ndjson'
    local APM_SERVER_INTAKE=${APM_SERVER_URL}/intake/v2/events
    local INTAKE_DATA="../../../testdata/intake-v2/events.ndjson"

    echo "-> Sending events to APM Server..."
    # Return non zero if curl fails
    curl --fail --data-binary @${INTAKE_DATA} -H "${APM_AUTH_HEADER}" -H "${INTAKE_HEADER}" ${APM_SERVER_INTAKE}

    # TODO(marclop). It would be best to query Elasticsearch until at least X documents have been ingested.
    sleep 10
}

legacy_assert_events() {
    local INDEX="apm-${1}"
    local VERSION=${1}
    local ENTRIES=${2}
    assert_document "${INDEX}-error-*" "error.id" "9876543210abcdeffedcba0123456789" ${VERSION} ${ENTRIES}
    assert_document "${INDEX}-span-*" "span.id" "1234567890aaaade" ${VERSION} ${ENTRIES}
    assert_document "${INDEX}-transaction-*" "transaction.id" "4340a8e0df1906ecbfa9" ${VERSION} ${ENTRIES}
    assert_document "${INDEX}-metric-*" "transaction.type" "request" ${VERSION} ${ENTRIES}
}

data_stream_assert_events() {
    local TRACES_INDEX="traces-apm-*"
    local ERRORS_INDEX="logs-apm.error-*"
    local METRICS_INDEX="metrics-apm.internal-*"
    local VERSION=${1}
    local ENTRIES=${2}
    assert_document ${ERRORS_INDEX} "error.id" "9876543210abcdeffedcba0123456789" ${VERSION} ${ENTRIES}
    assert_document ${TRACES_INDEX} "span.id" "1234567890aaaade" ${VERSION} ${ENTRIES}
    assert_document ${TRACES_INDEX} "transaction.id" "4340a8e0df1906ecbfa9" ${VERSION} ${ENTRIES}
    assert_document ${METRICS_INDEX} "transaction.type" "request" ${VERSION} ${ENTRIES}
}

healthcheck() {
    local PUBLISH_READY=$(curl -s --fail -H "${APM_AUTH_HEADER}" ${APM_SERVER_URL} | jq '.publish_ready')
    if [[ ! ${PUBLISH_READY} ]]; then
        local MAX_RETRIES=10
        if [[ ${1} -gt 0 ]] && [[ ${1} -lt ${MAX_RETRIES} ]]; then
            echo "-> APM Server isn't ready to receive events, retrying (${1}/${MAX_RETRIES})..."
            sleep $((1 * ${1}))
            healthcheck $((1 + ${1}))
            return
        else
            echo "-> APM Server isn't ready to receive events, maximum retries exceeded"
            exit 1
        fi
    else
        echo "-> APM Server ready!"
    fi
}

upgrade_managed() {
    local CURR_VERSION=${1}
    local AUTH=${ELASTICSEARCH_USER}:${ELASTICSEARCH_PASS}
    local URL_MIGRATE=${KIBANA_URL}/internal/apm/fleet/cloud_apm_package_policy

    echo "-> Upgrading APM Server ${CURR_VERSION} to managed mode..."
    local RESULT=$(curl -s --fail -H 'kbn-xsrf: true' -u "${AUTH}" -XPOST ${URL_MIGRATE})
    local ENABLED=$(echo ${RESULT} | jq '.cloudApmPackagePolicy.enabled')

    if [[ ! ${ENABLED} ]]; then
        echo "-> Failed migrating and enabling the APM Integration"
        exit 6
    fi  

    # Allow the new server to start serving requets. Waiting for an arbitrary 70 seconds
    # period is not ideal, but there aren't any other APIs we can query.
    echo "-> Waiting for 70 seconds for the APM Server to become available..."
    sleep 70
}
