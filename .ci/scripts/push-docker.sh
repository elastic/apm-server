#!/usr/bin/env bash
#
# Push the docker image for the apm-server to the given docker registry
#
# Arguments:
# - NEW_TAG, this is the tag for the docker image to be pushed.
# - NEW_IMAGE, this is the docker image name to be pushed.
#
set -euo pipefail

NEW_TAG=${1:?Docker tag is not set}
NEW_IMAGE=${2:?Docker image is not set}

export IMAGE="docker.elastic.co/apm/apm-server"

echo 'INFO: Get the just built docker image'
TAG=$(docker images ${IMAGE} --format "{{.Tag}}" | head -1)

echo "INFO: Retag docker image (${IMAGE}:${TAG})"
docker tag "${IMAGE}:${TAG}" "${NEW_IMAGE}:${NEW_TAG}"

echo "INFO: Push docker image (${NEW_IMAGE}:${NEW_TAG})"
docker push "${NEW_IMAGE}:${NEW_TAG}"
