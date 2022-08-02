#!/usr/bin/env bash

# shellcheck disable=SC1091
source /usr/local/bin/bash_standard_lib.sh

# shellcheck disable=SC1091
source ./script/common.bash

get_go_version

DOCKER_IMAGES="
docker.elastic.co/infra/release-manager:latest
golang:${GO_VERSION}
"
if [ -x "$(command -v docker)" ]; then
  for image in ${DOCKER_IMAGES}
  do
  (retry 2 docker pull "${image}") || echo "Error pulling ${image} Docker image, we continue"
  done
fi
