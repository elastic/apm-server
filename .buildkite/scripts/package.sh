#!/usr/bin/env bash
##
##  Builds the distribution packages: tarballs, RPMs, Docker images, etc.
##  It runs for both linux/amd64 and linux/arm64. On linux/amd64 it builds all packages;
##  on linux/arm64 it builds only Docker images.
##

set -eo pipefail

TYPE="$1"
PLATFORM_TYPE=$(uname -m)

PLATFORMS=''
PACKAGES=''
if [[ ${PLATFORM_TYPE} == "arm" || ${PLATFORM_TYPE} == "aarch64" ]]; then
  PLATFORMS='linux/arm64'
  PACKAGES='docker'
fi

MAKE_GOAL=release-manager-snapshot
if [[ ${TYPE} == "staging" ]]; then
  MAKE_GOAL="release-manager-release"
fi

PLATFORMS=$PLATFORMS PACKAGES=$PACKAGES \
make $MAKE_GOAL
