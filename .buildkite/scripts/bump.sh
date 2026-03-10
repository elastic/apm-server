#!/usr/bin/env bash
##
## This script is responsible for triggering the GitHub Actions workflow to bump the version of the APM Server.
##
set -eo pipefail

# Either staging or snapshot
BRANCH="$1"
NEW_VERSION="$2"

if [[ -z "$BRANCH" || -z "$NEW_VERSION" ]]; then
  echo "Usage: $0 <branch> <new_version>"
  exit 1
fi

workflow_name=run-patch-release.yml
if [[ "$NEW_VERSION" =~ \.0$ ]]; then
  workflow_name=run-minor-release.yml
else
  echo "Bumping patch version for branch $BRANCH to $NEW_VERSION"
fi

# 1. Trigger the workflow with inputs (if needed)
gh workflow run "$workflow_name" \
  --field version="$NEW_VERSION" \
  --repo elastic/apm-server

# 2. Get the run ID of the triggered workflow
sleep 5  # Give GitHub a moment to create the run
RUN_ID=$(gh run list --workflow="$workflow_name" --limit 1 --json databaseId --jq '.[0].databaseId')

# 3. Watch and wait for completion
gh run watch "$RUN_ID"

# 4. View the final output
gh run view "$RUN_ID"
