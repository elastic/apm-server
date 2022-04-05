#!/usr/bin/env bash

# shellcheck disable=SC1091
source /usr/local/bin/bash_standard_lib.sh

# shellcheck disable=SC1091
source ./script/common.bash

get_go_version

DOCKER_IMAGES="
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-arm
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-darwin
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-main
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-main-debian7
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-main-debian8
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-mips
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-ppc
docker.elastic.co/beats-dev/golang-crossbuild:${GO_VERSION}-s390x
docker.elastic.co/infra/release-manager:latest
golang:${GO_VERSION}
"
if [ -x "$(command -v docker)" ]; then
  for image in ${DOCKER_IMAGES}
  do
  (retry 2 docker pull "${image}") || echo "Error pulling ${image} Docker image, we continue"
  done
fi

echo "Fetch all the required toolchain (docker images) that might change overtime"
make release-manager-snapshot || true

if docker version >/dev/null ; then
  ## Detect architecture to support ARM specific docker images.
  ARCH=$(uname -m| tr '[:upper:]' '[:lower:]')
  DOCKER_IMAGE=alpine:3.4
  if [ "${ARCH}" == "aarch64" ] ; then
    DOCKER_IMAGE=arm64v8/alpine:3
  fi
  set -e
  echo "Change ownership of all files inside the specific folder from root/root to current user/group"
  set -x
  docker run -v "$(pwd)":/beat ${DOCKER_IMAGE} sh -c "find /beat -user 0 -exec chown -h $(id -u):$(id -g) {} \;" || true
fi

echo "Change permissions with write access of all files"
chmod -R +w . || true
