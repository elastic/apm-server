#!/usr/bin/env bash
#
# Generate a json array containing branches to be processed.
# It is used to run smoke tests on the default branch, the latest release branch 
# and the unreleased branch if exist in FF phase.

# Bash strict mode
set -eo pipefail
trap 's=$?; echo >&2 "$0: Error on line "$LINENO": $BASH_COMMAND"; exit $s' ERR

# List all 8.x branches from most recent to older.
# Note: We need to flatten output results to avoid multiples arrays due to pagination.
# Ref: https://github.com/cli/cli/issues/1268
branches=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  /repos/elastic/apm-server/branches \
  --paginate \
  -q 'map(select(.name | startswith("8."))) | sort_by(.name) | map(.name) | reverse' | jq -s 'flatten')

# Retrieve latest release
latest_release=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  /repos/elastic/apm-server/releases \
  --paginate \
  -q 'map(select(.tag_name | startswith("v8."))) | sort_by(.tag_name) | map(.tag_name) | .[-1] | ltrimstr("v")')

# Extract major.minor semver from major.minor.patch semver
latest_release_major_minor=$(echo "\"${latest_release}\"" | jq -r '.[:-2]')

# Retrieve current index by matching latest release and branch
current_branch_idx=$(echo "${branches}" | jq "indices(\"${latest_release_major_minor}\") | .[0]")

# Extract all branches to process
echo "${branches}" | jq -c ".[:$((current_branch_idx + 1))] | . += [\"main\"]"
