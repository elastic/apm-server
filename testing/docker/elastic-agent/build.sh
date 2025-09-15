#!/usr/bin/bash

# This script builds an image from the elastic-agent image
# with a locally built apm-server binary injected. Additional
# flags (e.g. -t <name>) will be passed to `docker build`.

set -eu

REPO_ROOT=$(cd $(dirname $(readlink -f "$0"))/../../.. && pwd)

DEFAULT_IMAGE_TAG=$(grep docker.elastic.co/kibana ${REPO_ROOT}/docker-compose.yml | cut -d: -f3)
BASE_IMAGE="${BASE_IMAGE:-docker.elastic.co/beats/elastic-agent:$DEFAULT_IMAGE_TAG}"

export DOCKER_BUILDKIT=1
docker pull $BASE_IMAGE

STACK_VERSION=$(docker inspect -f '{{index .Config.Labels "org.label-schema.version"}}' $BASE_IMAGE)
VCS_REF=$(docker inspect -f '{{index .Config.Labels "org.label-schema.vcs-ref"}}' $BASE_IMAGE)

make -C $REPO_ROOT build/apm-server-linux-amd64
make -C $REPO_ROOT build/apm-server-linux-arm64

docker buildx create \
  --name elastic-agent-container-builder \
  --driver docker-container \
  --bootstrap --use

docker buildx build \
	-f $REPO_ROOT/testing/docker/elastic-agent/Dockerfile \
	--build-arg ELASTIC_AGENT_IMAGE=$BASE_IMAGE \
	--build-arg STACK_VERSION=$STACK_VERSION \
	--build-arg VCS_REF_SHORT=${VCS_REF:0:6} \
	--platform linux/amd64,linux/arm64 \
	--push \
	$* $REPO_ROOT/build

docker buildx rm elastic-agent-container-builder
