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

<<<<<<< HEAD
PLATFORMS=$PLATFORMS PACKAGES=$PACKAGES \
=======
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
>>>>>>> ffba995e (dra: use a google bucket that contains the elastic qualifier (#15350))
make $MAKE_GOAL

# Context switch back to the previous workspace
# so the following steps and archive in Buidlkite work as expected
cp -rf . ../apm-server-package
