#!/usr/bin/env bash
set -euo pipefail

STEP=${1:-""}

DOCKER_INFO_DIR="build/docker-info/${STEP}"
mkdir -p ${DOCKER_INFO_DIR}
cp docker-compose*.yml ${DOCKER_INFO_DIR}
cd ${DOCKER_INFO_DIR}

docker ps -a &> docker-containers.txt

DOCKER_IDS=$(docker ps -aq)

for id in ${DOCKER_IDS}
do
  docker ps -af id=${id} --no-trunc &> ${id}-cmd.txt
  docker logs ${id} &> ${id}.log || echo "It is not possible to grab the logs of ${id}"
  docker inspect ${id} &> ${id}-inspect.json || echo "It is not possible to grab the inspect of ${id}"
done
