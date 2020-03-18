#!/usr/bin/env bash
#
# Build the docker image for the apm-server, retag and push it to the given docker registry
#
# Arguments:
# - NEW_TAG
# - NEW_IMAGE
#
set -euo pipefail

NEW_TAG=${1:?Docker tag is not set}
NEW_IMAGE=${1:?Docker image is not set}

export PLATFORMS='linux/amd64'
export TYPE='docker'
export SNAPSHOT='true'
export IMAGE="docker.elastic.co/apm/apm-server"

echo 'INFO: Build docker images'
mage package

echo 'INFO: Get the just built docker image'
TAG=$(docker images ${IMAGE} --format "{{.Tag}}" | head -1)

echo "INFO: Retag docker image (${IMAGE}:${TAG})"
docker tag "${IMAGE}:${TAG}" "${NEW_IMAGE}:${NEW_TAG}"

echo "INFO: Push docker image (${NEW_IMAGE}:${NEW_TAG})"
docker push "${NEW_IMAGE}:${NEW_TAG}"
