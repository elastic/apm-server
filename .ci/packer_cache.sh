#!/usr/bin/env bash

# shellcheck disable=SC1091
source /usr/local/bin/bash_standard_lib.sh

# shellcheck disable=SC1091
source ./script/common.bash

# This script runs within the Jenkins context, but for some reason
# WORKSPACE is not available, this should bypass the requirement set
# when in ./script/common.bash
if [ -z "${WORKSPACE}" ] ; then
	echo "WARN: WORKSPACE env variable is empty, hence creating a temporary dir"
	WORKSPACE=$(mktemp -d)
	export WORKSPACE
	echo "INFO: WORKSPACE=${WORKSPACE}"
fi

jenkins_setup

# Fetch Docker images used for packaging.
DOCKER_IMAGES="
docker.elastic.co/infra/release-manager:latest
golang:1.19.5
"
if [ -x "$(command -v docker)" ]; then
  for image in ${DOCKER_IMAGES}
  do
  (retry 2 docker pull "${image}") || echo "Error pulling ${image} Docker image, we continue"
  done
fi

# Download Go module dependencies.
GO_MOD_DIRS=". tools systemtest"
for dir in ${GO_MOD_DIRS}; do
  (cd $dir && go mod download) || echo "Error downloading modules from ${dir}/go.mod, continuing"
done
