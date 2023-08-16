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

### TODO: fetch all the generated artifacts
# BK artifact API call

### TODO: retry a few times just in case some infra issues, like the vault accessing.
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
