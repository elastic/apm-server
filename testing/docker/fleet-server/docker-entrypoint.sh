#!/bin/bash

SERVICE_TOKEN_FILE=service_token

if [[ ! -f ${SERVICE_TOKEN_FILE} ]]; then
    jsonBody="$(curl -u${KIBANA_USERNAME}:${KIBANA_PASSWORD} -fsSL -XPOST \
        "${FLEET_SERVER_ELASTICSEARCH_HOST}/_security/service/elastic/fleet-server/credential/token/compose")"
    # use grep and sed to get the service token value as we may not have jq or a similar tool on the instance
    token=$(echo ${jsonBody} |  grep -Eo '"value"[^}]*' | grep -Eo ':.*' | sed -r "s/://" | sed -r 's/"//g')
    
    echo ${token} > ${SERVICE_TOKEN_FILE}
fi

export FLEET_SERVER_SERVICE_TOKEN=$(cat ${SERVICE_TOKEN_FILE})

/usr/bin/tini -s -- /usr/local/bin/docker-entrypoint
