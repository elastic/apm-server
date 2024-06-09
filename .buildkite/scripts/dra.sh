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

## Read current version.
VERSION=$(make get-version)

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
fi

dra() {
  local workflow=$1
  echo "--- Prepare release manager $workflow"
  .ci/scripts/prepare-release-manager.sh $workflow

  echo "--- Run release manager $workflow"
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

  # Create Buildkite annotation similarly done in Beats:
  # https://github.com/elastic/beats/blob/90f9e8f6e48e76a83331f64f6c8c633ae6b31661/.buildkite/scripts/dra.sh#L74-L81
  if [[ "$command" == "collect" ]]; then
    # extract the summary URL from a release manager output line like:
    # Report summary-18.22.0.html can be found at https://artifacts-staging.elastic.co/apm-server/18.22.0-ABCDEFGH/summary-18.22.0.html
    SUMMARY_URL=$(grep -E '^Report summary-.* can be found at ' rm-output.txt | grep -oP 'https://\S+' | awk '{print $1}')
    rm rm-output.txt

    # and make it easily clickable as a Builkite annotation
    printf "**${workflow} summary link:** [${SUMMARY_URL}](${SUMMARY_URL})\n" | buildkite-agent annotate --style=success --append
  fi
}

dra "snapshot"
if [[ "${BUILDKITE_BRANCH}" != "main" ]]; then
  dra "staging"
fi
