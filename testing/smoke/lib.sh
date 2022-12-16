#!/bin/bash

export TF_IN_AUTOMATION=1
export TF_CLI_ARGS=-no-color

get_versions() {
    if [[ -z ${EC_API_KEY} ]]; then
        echo "-> ESS API Key not set, please set the EC_API_KEY environment variable."
        return 1
    fi
    # initialize terraform, so we can obtain the configured region.
    terraform_init

    local REGION=$(echo var.region | terraform console | tr -d '"')
    local EC_VERSION_ENDPOINT="https://cloud.elastic.co/api/v1/regions/${REGION}/stack/versions?show_deleted=false&show_unusable=false"
    local RES=$(curl_fail -H "Authorization: ApiKey ${EC_API_KEY}" ${EC_VERSION_ENDPOINT})
    if [ "$?" -ne 0 ]; then echo "${RES}\n"; fi
    VERSIONS=$(echo "${RES}" | jq -r -c '[.stacks[].version | select(. | contains("-") | not)] | sort')
}

get_latest_patch() {
    LATEST_PATCH=$(echo ${VERSIONS} | jq -r -c "[.[] | select(. |startswith(\"${1}\"))] | last" | cut -d '.' -f3)
}

get_latest_snapshot() {
    if [[ -z ${EC_API_KEY} ]]; then
        echo "-> ESS API Key not set, please set the EC_API_KEY environment variable."
        return 1
    fi
    # initialize terraform, so we can obtain the configured region.
    terraform_init

    local REGION=$(echo var.region | terraform console | tr -d '"')
    local EC_VERSION_ENDPOINT="https://cloud.elastic.co/api/v1/regions/${REGION}/stack/versions?show_deleted=false&show_unusable=true"
    local RES=$(curl_fail -H "Authorization: ApiKey ${EC_API_KEY}" ${EC_VERSION_ENDPOINT})
    if [ "$?" -ne 0 ]; then echo "${RES}\n"; fi
    VERSIONS=$(echo "${RES}" | jq -r -c '[.stacks[].version | select(. | contains("-"))] | sort')
}

terraform_init() {
    if [[ ! -f main.tf ]]; then cp ../main.tf .; fi
    terraform init >> tf.log
    terraform validate >> tf.log
}

append_tfvar() {
    if [[ -f terraform.tfvars && ! -z ${3} && ${3} -gt 0 ]]; then rm terraform.tfvars; fi
    echo ${1}=\"${2}\" >> terraform.tfvars
}

terraform_apply() {
    echo "-> Creating / Upgrading deployment to version ${1}"
    if [[ ! -z ${1} ]]; then echo stack_version=\"${1}\" > terraform.tfvars; fi
    if [[ ! -z ${2} ]]; then echo integrations_server=${2} >> terraform.tfvars; fi
    terraform apply -auto-approve >> tf.log

    if [[ ${EXPORTED_AUTH} ]]; then
        return
    fi
    ELASTICSEARCH_URL=$(terraform output -raw elasticsearch_url)
    ELASTICSEARCH_USER=$(terraform output -raw elasticsearch_username)
    ELASTICSEARCH_PASS=$(terraform output -raw elasticsearch_password)
    APM_AUTH_HEADER="Authorization: Bearer $(terraform output -raw apm_secret_token)"
    APM_SERVER_URL=$(terraform output -raw apm_server_url)
    KIBANA_URL=$(terraform output -raw kibana_url)
    STACK_VERSION=$(terraform output -raw stack_version)
    EXPORTED_AUTH=true
}

terraform_destroy() {
    exit_code=$?
    if [[ ${exit_code} -gt 0 ]]; then
        echo "-> Smoke tests FAILED!!"
        echo "-> Printing terraform logs:"
        cat tf.log
    fi
    echo "-> Destroying the underlying infrastructure..." 
    terraform destroy -auto-approve >> tf.log
    rm -f terraform.tfvars tf.log
    if [[ -z ${1} || ${1} -gt 0 ]]; then rm -f main.tf; fi
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
    RESULT=$(curl_fail -u ${AUTH} -XGET "${URL}" -H 'Content-Type: application/json' -d"{\"query\":{\"bool\":{\"must\":[{\"match\":{\"${FIELD}\":\"${VALUE}\"}},{\"match\":{\"observer.version\":\"${VERSION}\"}}]}}}")
    if [ "$?" -ne 0 ]; then echo "${RESULT}\n"; fi

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
    curl_fail --data-binary @${INTAKE_DATA} -H "${APM_AUTH_HEADER}" -H "${INTAKE_HEADER}" ${APM_SERVER_INTAKE}

    # TODO(marclop). It would be best to query Elasticsearch until at least X documents have been ingested.
    sleep 10
}

delete_all() {
    local AUTH=${ELASTICSEARCH_USER}:${ELASTICSEARCH_PASS}
    local URL=${ELASTICSEARCH_URL}/_all

    echo "-> Removing all data from ES..."

    curl_fail -u ${AUTH} -XDELETE "${URL}"
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
    local RES=$(curl_fail -H "${APM_AUTH_HEADER}" ${APM_SERVER_URL})
    if [ "$?" -ne 0 ]; then echo "${RES}\n"; fi
    local PUBLISH_READY=$(echo ${RES}| jq '.publish_ready')
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
    local RESULT=$(curl_fail -H 'kbn-xsrf: true' -u "${AUTH}" -XPOST ${URL_MIGRATE})
    if [ "$?" -ne 0 ]; then echo "${RESULT}\n"; fi
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

legacy_assert_templates() {
    local VERSION=$1
    local TEMPLATE_PREFIX=apm-${VERSION}
    echo "-> Asserting legacy index templates..."
    # Verify the apm-${VERSION} template which contains the mappings
    elasticsearch_curl "/_template/${TEMPLATE_PREFIX}" > /dev/null

    # Then assert each individual template has the right ILM policies set.
    local TEMPLATES=( error metric profile span transaction )
    local SUCCESS=true
    for suffix in "${TEMPLATES[@]}"; do
        local TEMPLATE_NAME=${TEMPLATE_PREFIX}-${suffix}
        local RESPONSE=$(elasticsearch_curl "/_template/${TEMPLATE_NAME}")
        if [ "$?" -ne 0 ]; then echo "${RESPONSE}\n"; fi
        local ILM_POLICY=$(echo ${RESPONSE} | jq 'to_entries[0]|.value.settings.index.lifecycle')
        local ILM_POLICY_NAME=$(echo ${ILM_POLICY} | jq -r '.name')
        local ILM_POLICY_ROLLOVER_ALIAS=$(echo ${ILM_POLICY} | jq -r '.rollover_alias')
        if [[ "${ILM_POLICY_NAME}" != "${LEGACY_ILM_POLICY}" ]]; then
            echo "-> Invalid template ${TEMPLATE_NAME}; ILM policy name, expected ${LEGACY_ILM_POLICY}, got ${ILM_POLICY_NAME}"
            SUCCESS=false
        fi
        if [[ "${ILM_POLICY_ROLLOVER_ALIAS}" != "${TEMPLATE_NAME}" ]]; then
            echo "-> Invalid template ${TEMPLATE_NAME}; ILM policy rollover_alias, expected ${TEMPLATE_NAME}, got ${ILM_POLICY_ROLLOVER_ALIAS}"
            SUCCESS=false
        fi
    done
    if [[ ${SUCCESS} == false ]]; then
        echo "-> Failed asserting legacy templates"
        return 21
    fi
}

legacy_assert_ilm() {
    # Expected values
    local EXPECTED_MAX_SIZE=50gb
    local EXPECTED_MAX_AGE=30d
    local EXPECTED_PHASES=1
    echo "-> Asserting legacy ILM policies..."
    # Verify the ILM policy exists, and has the right settings.
    local RESPONSE=$(elasticsearch_curl "/_ilm/policy/${LEGACY_ILM_POLICY}")
    if [ "$?" -ne 0 ]; then echo "${RESPONSE}\n"; fi
    local ILM_PHASES=$(echo ${RESPONSE} | jq 'to_entries[0]|.value.policy.phases')
    local ILM_PHASE_COUNT=$(echo ${RESPONSE} | jq -r '. | length')
    local SUCCESS=true
    if [[ ${ILM_PHASE_COUNT} -ne ${EXPECTED_PHASES} ]]; then
        echo "-> Invalid ILM policy ${LEGACY_ILM_POLICY}; expected 1 phase got ${ILM_PHASE_COUNT}"
        echo "${ILM_PHASES}"
        SUCCESS=false
    fi
    local ILM_HOT_PHASE=$(echo ${ILM_PHASES} | jq '.hot')
    local ILM_HOT_PHASE_MAX_SIZE=$(echo ${ILM_HOT_PHASE} | jq -r '.actions.rollover.max_size')
    local ILM_HOT_PHASE_MAX_AGE=$(echo ${ILM_HOT_PHASE} | jq -r '.actions.rollover.max_age')
    if [[ "${ILM_HOT_PHASE_MAX_SIZE}" != "${EXPECTED_MAX_SIZE}" ]]; then
        echo "-> Invalid ILM policy ${LEGACY_ILM_POLICY}; expected hot phase max_size ${EXPECTED_MAX_SIZE} got ${ILM_HOT_PHASE_MAX_SIZE}"
        echo "${ILM_HOT_PHASE}"
        SUCCESS=false
    fi
    if [[ "${ILM_HOT_PHASE_MAX_AGE}" != "${EXPECTED_MAX_AGE}" ]]; then
        echo "-> Invalid ILM policy ${LEGACY_ILM_POLICY}; expected hot phase max_size ${EXPECTED_MAX_AGE} got ${ILM_HOT_PHASE_MAX_AGE}"
        echo "${ILM_HOT_PHASE}"
        SUCCESS=false
    fi
    if [[ ${SUCCESS} == false ]]; then
        echo "-> Failed asserting ILM policies"
        return 22
    fi
}

legacy_ingest_pipelines() {
    echo "-> Asserting legacy ingest pipelines..."
    local RESPONSE=$(elasticsearch_curl "/_ingest/pipeline/apm")
    if [ "$?" -ne 0 ]; then echo "${RESPONSE}\n"; fi
    local PIPELINES=( $(echo ${RESPONSE} | jq -r '.apm.processors[].pipeline.name') )
    local SUCCESS=true
    for pipeline in "${PIPELINES[@]}"; do
        # Verify the pipeline exists.
        local RESPONSE=$(elasticsearch_curl "/_ingest/pipeline/${pipeline}")
        if [ "$?" -ne 0 ]; then echo "${RESPONSE}\n"; fi
        local PIPELINE_BODY=$(echo ${RESPONSE} | jq 'to_entries[0]|.value.processors')
        if [[ -z ${PIPELINE_BODY} ]]; then
            echo "-> Invalid ingest pipeline ${pipeline}"
            echo "${RESPONSE}"
            SUCCESS=false
        fi
    done
    if [[ ${SUCCESS} == false ]]; then
        echo "-> Failed asserting ingest pipelines"
        return 23
    fi
}

legacy_assertions() {
    local VERSION=${1}
    local ENTRIES=${2}
    local LEGACY_ILM_POLICY=apm-rollover-30-days
    legacy_ingest_pipelines
    legacy_assert_templates ${VERSION}
    legacy_assert_ilm
    legacy_assert_events ${VERSION} ${ENTRIES}
}

data_stream_assertions() {
    local VERSION=${1}
    local ENTRIES=${2}
    data_stream_assert_templates_ilm ${VERSION}
    data_stream_assert_pipelines
    data_stream_assert_events ${VERSION} ${ENTRIES}
}

data_stream_assert_pipelines() {
    # NOTE(marclop) we could assert that the pipelines have some sort of version suffix
    # in their name, however, the APM package version may not equal the deployment version.
    echo "-> Asserting ingest pipelines..."
    local RESPONSE=$(elasticsearch_curl '/_ingest/pipeline/*apm*')
    if [ "$?" -ne 0 ]; then echo "${RESPONSE}\n"; fi
    local HAS_APM_PIPELINES=$(echo ${RESPONSE} | jq -r '.|length>0')
    if [[ ${HAS_APM_PIPELINES} != true ]]; then
        echo "-> Did not find any APM ingest pipelines"
        echo ${RESPONSE}
        return 31
    fi
}

data_stream_assert_templates_ilm() {
    echo "-> Asserting component templates and ILM policies..."
    local SUCCESS=true
    local VERSION=${1}
    local MAJOR_VERSION=$(echo ${VERSION} | cut -d '.' -f1 )
    if [[ ${MAJOR_VERSION} -eq 7 ]]; then
        local COMPONENTS=( settings custom )
    else
        local COMPONENTS=( package custom )
    fi

    local RES=$(elasticsearch_curl "/_ilm/policy")
    if [ "$?" -ne 0 ]; then echo "${RES}\n"; fi
    local COMPOSABLE_TEMPLATES=( $(echo ${RES} | jq -r 'to_entries[]|select(.key|contains("apm"))|.value.in_use_by.composable_templates[]') )
    for ct in "${COMPOSABLE_TEMPLATES[@]}"; do
        for type in "${COMPONENTS[@]}"; do
            local RESPONSE=$(elasticsearch_curl "/_component_template/${ct}@${type}")
            if [ "$?" -ne 0 ]; then echo "${RESPONSE}\n"; fi
            local FOUND=$(echo ${RESPONSE} | jq '.component_templates|length==1')
            if [[ ${FOUND} != true ]]; then
                echo "-> Unable to find component template ${ct}@${type}"
                SUCCESS=false
            fi
            # Ensure the ILM lifecycle policy exists.
            local ILM_POLICY_NAME=$(echo | jq -r '.component_templates[0].template.settings.index.lifecycle.name')
	    local RES=$(elasticsearch_curl "/_ilm/policy/${ILM_POLICY_NAME}")
            if [ "$?" -ne 0 ]; then echo "${RES}\n"; fi
        done
    done

    if [[ ${SUCCESS} == false ]]; then
        echo "-> Failed asserting component templates"
        return 32
    fi
}

elasticsearch_curl() {
    local URL=${1}
    local AUTH=${ELASTICSEARCH_USER}:${ELASTICSEARCH_PASS}
    curl_fail -H 'Content-Type: application/json' -u ${AUTH} -XGET "${ELASTICSEARCH_URL}${URL}"
}

curl_fail() {
    if [ -z "${HAS_FAIL_WITH_BODY}" ]; then
        curl -s --fail-with-body example.com &>/dev/null
        HAS_FAIL_WITH_BODY=$?
    fi

    if [ "${HAS_FAIL_WITH_BODY}" -eq 0 ]; then
        curl -s --fail-with-body "${@}"
    else
        curl -s --fail "${@}"
    fi
}
