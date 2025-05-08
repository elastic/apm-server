#!/usr/bin/env bash
##
##  Builds the distribution packages: tarballs, RPMs, Docker images, etc.
##  It runs for both linux/amd64 and linux/arm64. On linux/amd64 it builds all packages;
##  on linux/arm64 it builds only Docker images.
##

set -eo pipefail

TYPE="$1"


MAKE_GOAL=release-manager-snapshot
if [[ ${TYPE} == "staging" ]]; then
  MAKE_GOAL="release-manager-release"
fi

# Prepare the context for using a different workspace
# so the package can use the right folder location.
# Buildkite uses the pipeline name for the workspace and
# it breaks the package build process.
cp -rf . ../apm-server
cd ../apm-server

PLATFORMS=$PLATFORMS PACKAGES=$PACKAGES \
make $MAKE_GOAL

# Context switch back to the previous workspace
# so the following steps and archive in Buidlkite work as expected
cp -rf . ../apm-server-package
