#!/usr/bin/env bash
##
##  It relies on the .buildkite/hooks/pre-command so the Vault and other tooling
##  are prepared automatically by buildkite.
##
##  Required environment variables passed when running the Buildkite pipeline:
##   * BRANCH_NAME
##   * DRA_WORKFLOW
##   * GITHUB_SHA
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

echo "--- Run release manager"
# TODO: as long as it does not run as part of the GitHub action integration, then let's stop here
exit 0
docker run --rm \
  --name release-manager \
  -e VAULT_ADDR="${VAULT_ADDR_SECRET}" \
  -e VAULT_ROLE_ID="${VAULT_ROLE_ID_SECRET}" \
  -e VAULT_SECRET_ID="${VAULT_SECRET}" \
  --mount type=bind,readonly=false,src=$(pwd),target=/artifacts \
  docker.elastic.co/infra/release-manager:latest \
    cli collect \
    --project apm-server \
    --branch $BRANCH_NAME \
    --commit $GITHUB_SHA \
    --workflow $DRA_WORKFLOW \
    --artifact-set main \
    --version $VERSION
