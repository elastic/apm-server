#!/usr/bin/env bash
##
##  Builds the distribution packages: tarballs, RPMs, Docker images, etc.
##  It runs for both linux/amd64 and linux/arm64. On linux/amd64 it builds all packages;
##  on linux/arm64 it builds only Docker images.
##

set -eo pipefail

TYPE="$1"
PLATFORM_TYPE=$(uname -m)

MAKE_GOAL=package
if [[ ${PLATFORM_TYPE} == "arm" || ${PLATFORM_TYPE} == "aarch64" ]]; then
  MAKE_GOAL=package-docker
fi

if [[ ${TYPE} == "snapshot" ]]; then
  MAKE_GOAL="${MAKE_GOAL}-snapshot"
fi

echo "--- Configure golang :golang:"
eval "$(./gvm $GO_VERSION)"
echo "Golang version:"
go version

echo "--- Run Make"
make $MAKE_GOAL

# BK artifact API call
