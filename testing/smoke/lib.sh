#!/usr/bin/env bash

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
    local RES
    local RC=0
    RES=$(curl_fail -H "Authorization: ApiKey ${EC_API_KEY}" ${EC_VERSION_ENDPOINT}) || RC=$?
    if [ $RC -ne 0 ]; then echo "${RES}"; fi
    # NOTE: jq with semver requires some numeric transformation with sort_by
    VERSIONS=$(echo "${RES}" | jq -r -c '[.stacks[].version | select(. | contains("-") | not)] | sort_by(.| split(".") | map(tonumber))')
}

get_latest_version() {
    if [[ -z "${VERSIONS}" ]]; then
        echo "-> Version not set, call get_versions first"
        return 1
    fi
    local version
    version=$(echo ${VERSIONS} | jq -r -c "max_by(. | select(. | startswith(\"${1}\")) | if endswith(\"-SNAPSHOT\") then .[:-9] else . end | split(\".\") | map(tonumber))")
    echo "${version}"
}

get_latest_patch() {
    if [[ -z "${1}" ]]; then
        echo "-> Version not set"
        return 1
    fi
    LATEST_PATCH=$(get_latest_version "${1}" | cut -d '.' -f3)
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
    local RES
    local RC=0
    RES=$(curl_fail -H "Authorization: ApiKey ${EC_API_KEY}" ${EC_VERSION_ENDPOINT}) || RC=$?
    if [ $RC -ne 0 ]; then echo "${RES}"; fi
    # NOTE: semver with SNAPSHOT is not working when using the sort_by function in jq,
    #       that's the reason for transforming the SNAPSHOT in a semver 4 digits.
    VERSIONS=$(echo "${RES}" | jq -r -c '[.stacks[].version | select(. | contains("-SNAPSHOT"))] | sort' | sed 's#-SNAPSHOT#.0#g' | jq -r -c ' sort_by(.| split(".") | map(tonumber))' | sed 's#.0"#-SNAPSHOT"#g' | jq -r -c .)
}

get_latest_snapshot_for_version() {
    if [[ -z "${1}" ]]; then
        echo "-> Version not set"
        return 1
    fi
    get_latest_snapshot
    LATEST_SNAPSHOT_VERSION=$(echo "$VERSIONS" | jq -r -c "map(select(. | startswith(\"${1}\"))) | .[-1]")
}

terraform_init() {
    if [[ ! -f main.tf ]]; then cp ../main.tf .; fi
    terraform init >> tf.log
    terraform validate >> tf.log
}

cleanup_tfvar() {
    if [[ -f terraform.tfvars ]]; then rm terraform.tfvars; fi
}

append_tfvar() {
    echo "-> Adding tfvar ${1}=\"${2}\""
    echo ${1}=\"${2}\" >> terraform.tfvars
}

terraform_apply() {
    echo "-> Applying terraform configuration..."
    terraform apply -auto-approve >> tf.log

    ELASTICSEARCH_URL=$(terraform output -raw elasticsearch_url)
    ELASTICSEARCH_USER=$(terraform output -raw elasticsearch_username)
    ELASTICSEARCH_PASS=$(terraform output -raw elasticsearch_password)
    APM_AUTH_HEADER="Authorization: Bearer $(terraform output -raw apm_secret_token)"
    APM_SERVER_URL=$(terraform output -raw apm_server_url)
    KIBANA_URL=$(terraform output -raw kibana_url)
    STACK_VERSION=$(terraform output -raw stack_version)
    APM_SERVER_IP=$(terraform output -raw apm_server_ip)
    APM_SERVER_SSH_USER=$(terraform output -raw apm_server_ssh_user)
    APM_SERVER_OS=$(terraform output -raw apm_server_os)
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

    local RC=0
    # RESULT needs to be a global variable in order to be able to parse
    # the whole result in assert_entry. Passing it as a string
    # argument doesn't work well.
    RESULT=$(curl_fail -u ${AUTH} -XGET "${URL}" -H 'Content-Type: application/json' -d"{\"query\":{\"bool\":{\"must\":[{\"match\":{\"${FIELD}\":\"${VALUE}\"}},{\"match\":{\"observer.version\":\"${VERSION}\"}}]}}}") || RC=$?
    if [ $RC -ne 0 ]; then echo "${RESULT}"; fi

    echo "-> Asserting ${INDEX} contains expected documents..."
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
        return 2
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
    sleep 5
}

delete_all() {
    local AUTH=${ELASTICSEARCH_USER}:${ELASTICSEARCH_PASS}
    local URL=${ELASTICSEARCH_URL}/*apm*/_delete_by_query?expand_wildcards=all

    echo "-> Removing all data from ES..."

    curl_fail -H 'Content-Type: application/json' -u ${AUTH} -XPOST ${URL} -d '{"query": {"match_all": {}}}'
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
    retry 6 assert_document ${ERRORS_INDEX} "error.id" "9876543210abcdeffedcba0123456789" ${VERSION} ${ENTRIES}
    retry 6 assert_document ${TRACES_INDEX} "span.id" "1234567890aaaade" ${VERSION} ${ENTRIES}
    retry 6 assert_document ${TRACES_INDEX} "transaction.id" "4340a8e0df1906ecbfa9" ${VERSION} ${ENTRIES}
    retry 6 assert_document ${METRICS_INDEX} "transaction.type" "request" ${VERSION} ${ENTRIES}
}

healthcheck() {
    local RES
    local RC=0
    RES=$(curl_fail -H "${APM_AUTH_HEADER}" ${APM_SERVER_URL}) || RC=$?
    if [ $RC -ne 0 ]; then echo "${RES}"; fi
    local PUBLISH_READY=$(echo ${RES}| jq '.publish_ready')
    if [[ "${PUBLISH_READY}" != "true" ]]; then
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
    local RESULT
    local RC=0
    RESULT=$(curl_fail -H 'kbn-xsrf: true' -u "${AUTH}" -XPOST ${URL_MIGRATE}) || RC=$?
    if [ $RC -ne 0 ]; then echo "${RESULT}"; fi
    local ENABLED=$(echo ${RESULT} | jq '.cloudApmPackagePolicy.enabled')

    if [[ ! ${ENABLED} ]]; then
        echo "-> Failed migrating and enabling the APM Integration"
        exit 6
    fi

    # Allow the new server to start serving requests. Waiting for an arbitrary 70 seconds
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
        local RESPONSE
        local RC=0
        RESPONSE=$(elasticsearch_curl "/_template/${TEMPLATE_NAME}") || RC=$?
        if [ $RC -ne 0 ]; then echo "${RESPONSE}"; fi
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
    local RESPONSE
    local RC=0
    RESPONSE=$(elasticsearch_curl "/_ilm/policy/${LEGACY_ILM_POLICY}") || RC=$?
    if [ $RC -ne 0 ]; then echo "${RESPONSE}"; fi
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
        echo "-> Invalid ILM policy \"${LEGACY_ILM_POLICY}\"; expected hot phase \"max_age\" ${EXPECTED_MAX_SIZE} got ${ILM_HOT_PHASE_MAX_SIZE}"
        echo "${ILM_HOT_PHASE}"
        SUCCESS=false
    fi
    if [[ "${ILM_HOT_PHASE_MAX_AGE}" != "${EXPECTED_MAX_AGE}" ]]; then
        echo "-> Invalid ILM policy \"${LEGACY_ILM_POLICY}\"; expected hot phase \"max_age\" ${EXPECTED_MAX_AGE} got ${ILM_HOT_PHASE_MAX_AGE}"
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
    local RESPONSE
    local RC=0
    RESPONSE=$(elasticsearch_curl "/_ingest/pipeline/apm") || RC=$?
    if [ $RC -ne 0 ]; then echo "${RESPONSE}"; fi
    local PIPELINES=( $(echo ${RESPONSE} | jq -r '.apm.processors[].pipeline.name') )
    local SUCCESS=true
    for pipeline in "${PIPELINES[@]}"; do
        # Verify the pipeline exists.
        local RESPONSE
        local RC=0
        RESPONSE=$(elasticsearch_curl "/_ingest/pipeline/${pipeline}") || RC=$?
        if [ $RC -ne 0 ]; then echo "${RESPONSE}"; fi
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

    local MAJOR MINOR
    MAJOR=$(echo $VERSION | cut -d. -f1)
    MINOR=$(echo $VERSION | cut -d. -f2)

    data_stream_assert_pipelines
    data_stream_assert_events ${VERSION} ${ENTRIES}
}

data_stream_assert_pipelines() {
    # NOTE(marclop) we could assert that the pipelines have some sort of version suffix
    # in their name, however, the APM package version may not equal the deployment version.
    echo "-> Asserting ingest pipelines..."
    local RESPONSE
    local RC=0
    RESPONSE=$(elasticsearch_curl '/_ingest/pipeline/*apm*') || RC=$?
    if [ $RC -ne 0 ]; then echo "${RESPONSE}"; fi
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

    local COMPOSABLE_TEMPLATES=($(elasticsearch_curl "/_component_template" | jq -c -r '.component_templates|to_entries[]|select(.value.component_template._meta.package.name == "apm")|.value'))
    for ct in "${COMPOSABLE_TEMPLATES[@]}"; do
        local CT_NAME=$(echo "${ct}" | jq -r '.name')
        local ILM_POLICY_NAME=$(echo "${ct}" | jq -r '.component_template.template.settings.index.lifecycle.name // empty')
        if [[ ! -z "${ILM_POLICY_NAME}" ]]; then
            # ignore tail based sampling ILM policy for 7.x
            if [ "${ILM_POLICY_NAME}" = "traces-apm.sampled-default_policy" ] && [ "${VERSION}" != "${VERSION#7.}" ]; then
                SUCCESS=true
            else
                # Component template has ILM policy attached, check if ILM policy actually exists.
                local RES
                local RC=0
                RES=$(elasticsearch_curl "/_ilm/policy/${ILM_POLICY_NAME}") || RC=$?
                if [ $RC -ne 0 ]; then
                    echo "-> ILM policy ${ILM_POLICY_NAME} error: ${RES}"
                    SUCCESS=false
                fi
            fi
        elif [[ $CT_NAME == *@package ]]; then
            # @package component templates should always have ILM policy.
            echo "-> Package component template ${CT_NAME} has no ILM policy"
            SUCCESS=false
        fi
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
    if is_curl_fail_with_body; then
        curl -s --fail-with-body "${@}"
    else
        curl -s --fail "${@}"
    fi
}

is_curl_fail_with_body() {
    if [ -z "${HAS_FAIL_WITH_BODY}" ]; then
        curl -s --fail-with-body example.com > /dev/null 2>&1
        HAS_FAIL_WITH_BODY=$?
    fi
    return $HAS_FAIL_WITH_BODY
}

retry() {
  local retries=$1
  shift

  local count=0
  until "$@"; do
    exit=$?
    wait=$((2 ** count))
    count=$((count + 1))
    if [ $count -lt "$retries" ]; then
      echo "-> Retry cmd: '$*';  $count/$retries exited $exit, retrying in $wait seconds..."
      sleep $wait
    else
      echo "-> Retry cmd: '$*'; $count/$retries exited $exit, no more retries left."
      return $exit
    fi
  done
  return 0
}

inspect_systemd_ulimits() {
    if [[ -z ${APM_SERVER_IP} ]] || [[ -z ${APM_SERVER_SSH_USER} ]] || [[ -z ${APM_SERVER_OS} ]]; then
        echo "-> Warning: Missing terraform outputs for ulimit inspection (IP, SSH user, or OS)"
        return 0
    fi

    if [[ -z ${KEY_NAME} ]]; then
        echo "-> Warning: Missing SSH key for ulimit inspection"
        return 0
    fi

    echo "-> Inspecting systemd ulimit values for apm-server on ${APM_SERVER_OS}..."
    echo "-> Connecting to ${APM_SERVER_SSH_USER}@${APM_SERVER_IP}"

    local SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=10"

    # Get apm-server PID
    local PID
    PID=$(ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "systemctl show apm-server --property MainPID --value 2>/dev/null || echo ''" 2>/dev/null || echo "")
    
    if [[ -z ${PID} ]] || [[ ${PID} == "0" ]]; then
        echo "-> Warning: Could not get apm-server PID from systemctl, trying pgrep..."
        PID=$(ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "pgrep -f apm-server | head -1 2>/dev/null || echo ''" 2>/dev/null || echo "")
    fi

    echo ""
    echo "=== Systemd Ulimit Inspection for ${APM_SERVER_OS} ==="
    echo ""

    # Systemd unit file limits
    echo "--- Systemd Unit File Limits ---"
    ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "systemctl show apm-server 2>/dev/null | grep -i limit || echo 'No limit settings found in unit file'" 2>/dev/null || echo "Failed to retrieve unit file limits"
    echo ""

    # Actual process limits (if PID is available)
    if [[ -n ${PID} ]] && [[ ${PID} != "0" ]]; then
        echo "--- Actual Process Limits (PID: ${PID}) ---"
        ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "cat /proc/${PID}/limits 2>/dev/null || echo 'Failed to read process limits'" 2>/dev/null || echo "Failed to retrieve process limits"
        echo ""
    else
        echo "--- Actual Process Limits ---"
        echo "Could not determine apm-server PID"
        echo ""
    fi

    # Systemd default limits
    echo "--- Systemd Default Limits ---"
    ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "systemctl show-defaults 2>/dev/null | grep -i limit || echo 'No default limit settings found'" 2>/dev/null || echo "Failed to retrieve systemd defaults"
    echo ""

    # Systemd system configuration
    echo "--- Systemd System Configuration ---"
    ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "grep -E '^[^#]*Limit' /etc/systemd/system.conf /etc/systemd/user.conf 2>/dev/null | grep -v '^#' || echo 'No limit settings in systemd config files'" 2>/dev/null || echo "Failed to read systemd config files"
    echo ""

    # OS-specific system limits
    echo "--- OS System Limits Configuration ---"
    ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "for f in /etc/security/limits.conf /etc/security/limits.d/*.conf; do [ -f \"\$f\" ] && echo \"=== \$f ===\" && grep -v '^#' \"\$f\" | grep -v '^$' || true; done" 2>/dev/null || echo "Failed to read limits configuration files"
    echo ""

    # Current ulimit for the apm-server user
    echo "--- Current Ulimit for apm-server User ---"
    ssh -i "${KEY_NAME}" ${SSH_OPTS} "${APM_SERVER_SSH_USER}@${APM_SERVER_IP}" "sudo -u apm-server sh -c 'ulimit -a' 2>/dev/null || echo 'Could not retrieve ulimit for apm-server user'" 2>/dev/null || echo "Failed to retrieve user ulimit"
    echo ""

    echo "=== End of Ulimit Inspection for ${APM_SERVER_OS} ==="
    echo ""
}
