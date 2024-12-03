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

# by default it uses the buildkite branch
DRA_BRANCH="$BUILDKITE_BRANCH"
# by default it publishes the DRA artifacts, for such it uses the collect command.
dra_command=collect
BRANCHES_URL=https://storage.googleapis.com/artifacts-api/snapshots/branches.json
curl -s "${BRANCHES_URL}" > active-branches.json
# as long as `8.x` is not in the active branches, we will explicitly add the condition.
if [ "$BUILDKITE_BRANCH" == "8.x" ] || grep -q "\"$BUILDKITE_BRANCH\"" active-branches.json ; then
  echo "--- :arrow_right: Release Manager only supports the current active branches and 8.x, running"
else
  # If no active branches are found, let's see if it is a feature branch.
  echo "--- :arrow_right: Release Manager only supports the current active branches, skipping"
  echo "BUILDKITE_BRANCH=$BUILDKITE_BRANCH"
  echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"
  echo "VERSION=$VERSION"
  echo "Supported branches:"
  cat active-branches.json
  if [[ $BUILDKITE_BRANCH =~ "feature/" ]]; then
    buildkite-agent annotate "${BUILDKITE_BRANCH} will list DRA artifacts. Feature branches are not supported. Look for the supported branches in ${BRANCHES_URL}" --style 'info' --context 'ctx-info'
    dra_command=list

    # use a different branch since DRA does not support feature branches but main/release branches
    # for such we will use the VERSION and https://storage.googleapis.com/artifacts-api/snapshots/<major.minor>.json
    # to know if the branch was branched out from main or the release branches.
    MAJOR_MINOR=${VERSION%.*}
    if curl -s "https://storage.googleapis.com/artifacts-api/snapshots/main.json" | grep -q "$VERSION" ; then
      DRA_BRANCH=main
    else
      if curl -s "https://storage.googleapis.com/artifacts-api/snapshots/$MAJOR_MINOR.json" | grep -q "$VERSION" ; then
        DRA_BRANCH="$MAJOR_MINOR"
      else
        buildkite-agent annotate "It was not possible to know the original base branch for ${BUILDKITE_BRANCH}. This won't fail - this is a feature branch." --style 'info' --context 'ctx-info-feature-branch'
        exit 0
      fi
    fi
  else
    buildkite-agent annotate "${BUILDKITE_BRANCH} is not supported yet. Look for the supported branches in ${BRANCHES_URL}" --style 'warning' --context 'ctx-warn'
    exit 1
  fi
fi

dra() {
  local workflow=$1
  local command=$2
  echo "--- Run release manager $workflow (DRA command: $command)"
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

dra "snapshot" "$dra_command"
if [[ "${DRA_BRANCH}" != "main" && "${DRA_BRANCH}" != "8.x" ]]; then
  echo "DRA_BRANCH is neither 'main' nor '8.x'"
  dra "staging" "$dra_command"
fi
