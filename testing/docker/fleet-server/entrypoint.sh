#!/bin/bash

DEST_PATH=$(compgen -G "/usr/share/elastic-agent/data/elastic-agent-*")
SOURCE_PATH="/fleet-server/apm-server"

if [[ -f ${SOURCE_PATH} ]]; then
    DEST_PATH=$(compgen -G "${DEST_PATH}/install/apm-server-*")
    if [[ -d ${DEST_PATH} ]]; then
        cp ${SOURCE_PATH} ${DEST_PATH}/apm-server
        echo "Using locally built APM Server"
    fi
fi

/usr/bin/tini -- /usr/local/bin/docker-entrypoint
