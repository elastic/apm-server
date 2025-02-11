#!/usr/bin/env bash
##
##  It relies on the .buildkite/hooks/pre-command so the Vault and other tooling
##  are prepared automatically by buildkite.
##
##  It downloads the generated artifacts and run the DRA only if the branch is an active
##  branch, based on the Unified Release policy. Otherwise, it won't run the DRA but print
##  some traces and fail unless it's a feature branch then it will list the DRA artifacts.
##

set -eo pipefail

# Either staging or snapshot
TYPE="$1"

# NOTE: load the shared functions
# shellcheck disable=SC1091
source .buildkite/scripts/utils.sh

echo "--- Restoring Artifacts"
buildkite-agent artifact download "build/**/*" .
buildkite-agent artifact download "build/dependencies*.csv" .
# The dependencies file needs to be saved in the build/distributions folder
cp build/dependencies*.csv build/distributions/

echo "--- Changing permissions for the release manager"
sudo chown -R :1000 build/
ls -l build/distributions/

if [[ "${BUILDKITE_PULL_REQUEST:-false}" == "true" ]]; then
  echo "--- :arrow_right: Release Manager does not run on PRs, skipping"
  exit 0
fi

# by default it uses the buildkite branch
DRA_BRANCH="$BUILDKITE_BRANCH"
# by default it publishes the DRA artifacts, for such it uses the collect command.
DRA_COMMAND=collect
VERSION=$(make get-version-only)
BRANCHES_URL=https://storage.googleapis.com/artifacts-api/snapshots/branches.json
curl -s "${BRANCHES_URL}" > active-branches.json
if ! grep -q "\"$BUILDKITE_BRANCH\"" active-branches.json ; then
  # If no active branches are found, let's see if it is a feature branch.
  dra_process_other_branches
fi

echo "--- :arrow_right: Release Manager only supports the current active branches"
echo "BUILDKITE_BRANCH=$BUILDKITE_BRANCH"
echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"
echo "VERSION=$VERSION"
echo "Supported branches:"
cat active-branches.json

dra() {
  local workflow=$1
  local command=$2
  local qualifier=${3:-""}
  echo "--- Run release manager $workflow (DRA command: $command)"
  set -x
  docker run --rm \
    --name release-manager \
    -e VAULT_ADDR="${VAULT_ADDR_SECRET}" \
    -e VAULT_ROLE_ID="${VAULT_ROLE_ID_SECRET}" \
    -e VAULT_SECRET_ID="${VAULT_SECRET}" \
    --mount type=bind,readonly=false,src=$(pwd),target=/artifacts \
    docker.elastic.co/infra/release-manager:latest \
      cli "$command" \
      --project apm-server \
      --branch $DRA_BRANCH \
      --commit $BUILDKITE_COMMIT \
      --workflow $workflow \
      --artifact-set main \
      --qualifier "$qualifier" \
      --version $VERSION | tee rm-output.txt
  set +x

  create_annotation_dra_summary "$command" "$workflow" rm-output.txt
}

if [[ "${TYPE}" == "staging" ]]; then
  qualifier=$(fetch_elastic_qualifier "$DRA_BRANCH")
  # TODO: main and 8.x are not needed to run the DRA for staging
  #       but main is needed until we do alpha1 releases of 9.0.0
  if [[ "${DRA_BRANCH}" != "8.x" ]]; then
    dra "${TYPE}" "$DRA_COMMAND" "${qualifier}"
  fi
fi

if [[ "${TYPE}" == "snapshot" ]]; then
  # NOTE: qualifier is not needed for snapshots, let's unset it.
  dra "${TYPE}" "$DRA_COMMAND" ""
fi
