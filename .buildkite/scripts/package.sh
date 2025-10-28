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

if [[ ${TYPE} == "staging" ]]; then
  echo "--- Prepare the Elastic Qualifier"
  # NOTE: load the shared functions
  # shellcheck disable=SC1091
  source .buildkite/scripts/utils.sh
  dra_process_other_branches
  ELASTIC_QUALIFIER=$(fetch_elastic_qualifier "$DRA_BRANCH")
  export ELASTIC_QUALIFIER
fi

echo "--- Run $MAKE_GOAL for $DRA_BRANCH"
make $MAKE_GOAL

ls -l build/distributions/
