#!/bin/bash

set -e

CONTAINER_NAME=apm-server
GITROOT=$(git rev-parse --show-toplevel)

if [[ ! -z $(docker ps --filter=name=${CONTAINER_NAME} --format='{{.Names}}') ]]; then
    docker rm -f ${CONTAINER_NAME}
fi
curl -s localhost:5601 > /dev/null && docker-compose --profile=monitoring down || true

docker-compose up -d
cd ${GITROOT}/systemtest/cmd/runapm
go run main.go -keep -name ${CONTAINER_NAME} -var expvar_enabled=true -d -f
cd -

echo "-> Enabling agent monitoring..."
docker cp ${GITROOT}/testing/docker/apm-server/agent-monitoring.sh ${CONTAINER_NAME}:/
docker exec apm-server /agent-monitoring.sh
docker restart apm-server
sed "s/CONTAINER_NAME/${CONTAINER_NAME}/" ${GITROOT}/testing/docker/metricbeat/apm-server.yml.tpl > ${GITROOT}/testing/docker/metricbeat/apm-server.yml
docker-compose --profile monitoring up -d

echo "export ELASTIC_APM_SERVER_URL=http://localhost:$(docker inspect ${CONTAINER_NAME} --format='{{range $p, $conf := .NetworkSettings.Ports}}{{(index $conf 0).HostPort}}{{end}}')"
