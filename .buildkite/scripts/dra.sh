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

echo "--- Changing permissions for the release manager"
sudo chown -R :1000 build/

echo "--- Debug files"
ls -l build/distributions/
ls -l build/

if [[ "${BUILDKITE_PULL_REQUEST:-false}" == "false" ]]; then
  echo "Release Manager does not run on PRs, skipping"
  exit 0
fi

curl -s https://storage.googleapis.com/artifacts-api/snapshots/branches.json > active-branches.json
if ! grep -q "\"$BUILDKITE_BRANCH\"" active-branches.json ; then
  echo "Release Manager only supports the current active branches, skipping"
  echo "BUILDKITE_BRANCH=$BUILDKITE_BRANCH"
  echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"
  echo "VERSION=$VERSION"
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
