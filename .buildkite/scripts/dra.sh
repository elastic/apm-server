#!/usr/bin/env bash
##
##  It relies on the .buildkite/hooks/pre-command so the Vault and other tooling
##  are prepared automatically by buildkite.
##
##  It downloads the generated artifacts and run the DRA only if the branch is an active
##  branch, based on the Unified Release policy. Otherwise, it won't run the DRA but print
##  some traces.
##

set -eo pipefail

<<<<<<< HEAD
## Read current version.
VERSION=$(make get-version)
=======
# Either staging or snapshot
TYPE="$1"

# NOTE: load the shared functions
# shellcheck disable=SC1091
source .buildkite/scripts/utils.sh
>>>>>>> ffba995e (dra: use a google bucket that contains the elastic qualifier (#15350))

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

<<<<<<< HEAD
BRANCHES_URL=https://storage.googleapis.com/artifacts-api/snapshots/branches.json
curl -s "${BRANCHES_URL}" > active-branches.json
if ! grep -q "\"$BUILDKITE_BRANCH\"" active-branches.json ; then
  echo "--- :arrow_right: Release Manager only supports the current active branches, skipping"
  echo "BUILDKITE_BRANCH=$BUILDKITE_BRANCH"
  echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"
  echo "VERSION=$VERSION"
  echo "Supported branches:"
  cat active-branches.json
  buildkite-agent annotate "${BUILDKITE_BRANCH} is not supported yet. Look for the supported branches in ${BRANCHES_URL}" --style 'warning' --context 'ctx-warn'
  exit 1
=======
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
>>>>>>> ffba995e (dra: use a google bucket that contains the elastic qualifier (#15350))
fi

echo "--- :arrow_right: Release Manager only supports the current active branches"
echo "BUILDKITE_BRANCH=$BUILDKITE_BRANCH"
echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"
echo "VERSION=$VERSION"
echo "Supported branches:"
cat active-branches.json

dra() {
  local workflow=$1
<<<<<<< HEAD
  echo "--- Prepare release manager $workflow"
  .ci/scripts/prepare-release-manager.sh $workflow

  echo "--- Run release manager $workflow"
=======
  local command=$2
  local qualifier=${3:-""}
  echo "--- Run release manager $workflow (DRA command: $command)"
  set -x
>>>>>>> ffba995e (dra: use a google bucket that contains the elastic qualifier (#15350))
  docker run --rm \
    --name release-manager \
    -e VAULT_ADDR="${VAULT_ADDR_SECRET}" \
    -e VAULT_ROLE_ID="${VAULT_ROLE_ID_SECRET}" \
    -e VAULT_SECRET_ID="${VAULT_SECRET}" \
    --mount type=bind,readonly=false,src=$(pwd),target=/artifacts \
    docker.elastic.co/infra/release-manager:latest \
      cli collect \
      --project apm-server \
      --branch $BUILDKITE_BRANCH \
      --commit $BUILDKITE_COMMIT \
      --workflow $workflow \
      --artifact-set main \
      --version $VERSION | tee rm-output.txt
<<<<<<< HEAD

  # Create Buildkite annotation similarly done in Beats:
  # https://github.com/elastic/beats/blob/90f9e8f6e48e76a83331f64f6c8c633ae6b31661/.buildkite/scripts/dra.sh#L74-L81
  if [[ "$command" == "collect" ]]; then
    # extract the summary URL from a release manager output line like:
    # Report summary-18.22.0.html can be found at https://artifacts-staging.elastic.co/apm-server/18.22.0-ABCDEFGH/summary-18.22.0.html
    SUMMARY_URL=$(grep -E '^Report summary-.* can be found at ' rm-output.txt | grep -oP 'https://\S+' | awk '{print $1}')
    rm rm-output.txt
=======
  set +x
>>>>>>> ffba995e (dra: use a google bucket that contains the elastic qualifier (#15350))

  create_annotation_dra_summary "$command" "$workflow" rm-output.txt
}

<<<<<<< HEAD
dra "snapshot"
if [[ "${BUILDKITE_BRANCH}" != "main" ]]; then
  dra "staging"
=======
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
>>>>>>> ffba995e (dra: use a google bucket that contains the elastic qualifier (#15350))
fi
