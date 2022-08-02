#!/usr/bin/env bash

# shellcheck disable=SC1091
source /usr/local/bin/bash_standard_lib.sh

# shellcheck disable=SC1091
source ./script/common.bash

jenkins_setup

# Fetch Docker images used for packaging.
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

# Download Go module dependencies.
go mod download
go mod download -modfile=tools/go.mod
go mod download -modfile=systemtest/go.mod
