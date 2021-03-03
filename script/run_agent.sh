#!/bin/bash

# Builds an apm-server Linux binary and runs an Elastic Agent image bind-mounting apm-server in the correct directory
# It expects an Elastic Agent image to exist and apm-integration-testing running (Elasticsearch and Kibana)

set -xe

version=$(mage version)

apmdir=apm-server-$version-linux-x86_64
builddir=build/distributions/$apmdir
mkdir -p $builddir
cp -f LICENSE.txt NOTICE.txt README.md apm-server.yml $builddir
GOOS=linux go build -o $builddir/apm-server ./x-pack/apm-server

imageid=$(docker image ls docker.elastic.co/beats/elastic-agent:"$version" -q)
sha=$(docker inspect "$imageid" | grep org.label-schema.vcs-ref | awk -F ": \"" '{print $2}' | head -c 6)
dst="/usr/share/elastic-agent/data/elastic-agent-${sha}/install/${apmdir}"

docker run --name elastic-agent-local -it \
  --env FLEET_ENROLL=1 \
  --env FLEET_ENROLL_INSECURE=1 \
  --env FLEET_SETUP=1 \
  --env KIBANA_HOST="http://admin:changeme@kibana:5601" \
  --env KIBANA_PASSWORD="changeme" \
  --env KIBANA_USERNAME="admin" \
  --network apm-integration-testing \
  -v "$(pwd)/$builddir:$dst" \
  -p 8200:8200 --rm "${imageid}"
