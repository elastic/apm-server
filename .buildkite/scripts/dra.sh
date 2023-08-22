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

if [[ "${BUILDKITE_PULL_REQUEST:-false}" == "false" ]]; then
  echo "--- :arrow_right: Release Manager does not run on PRs, skipping"
  exit 0
fi

curl -s https://storage.googleapis.com/artifacts-api/snapshots/branches.json > active-branches.json
if ! grep -q "\"$BUILDKITE_BRANCH\"" active-branches.json ; then
  echo "--- :arrow_right: Release Manager only supports the current active branches, skipping"
  echo "BUILDKITE_BRANCH=$BUILDKITE_BRANCH"
  echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"
  echo "VERSION=$VERSION"
  echo "Supported branches:"
  cat active-branches.json
  exit 0
fi

dra() {
  local workflow=$1
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
      --version $VERSION
}

dra "snapshot"
if [[ "${BUILDKITE_BRANCH}" != "main" ]]; then
  dra "staging"
fi
