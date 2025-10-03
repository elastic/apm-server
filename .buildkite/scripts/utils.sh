# An opinionated approach to manage the Elatic Qualifier for the DRA in a Google Bucket
# Instead of using the ELASTIC_QUALIFIER env variable.
fetch_elastic_qualifier() {
  local branch=$1
  qualifier=""
  URL="https://storage.googleapis.com/dra-qualifier/$branch"
  if curl -sf -o /dev/null "$URL" ; then
    qualifier=$(curl -s "$URL")
  fi
  echo "$qualifier"
}

retry() {
  local retries=$1
  shift

  local count=0
  until "$@"; do
    exit=$?
    wait=$((2 ** count))
    count=$((count + 1))
    if [ $count -lt "$retries" ]; then
      >&2 echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
      sleep $wait
    else
      >&2 echo "Retry $count/$retries exited $exit, no more retries left."
      return $exit
    fi
  done
  return 0
}

#
# An opinionated approach to detect if unsupported Unified Release branches
# can be used, this is handy for testing feature branches in dry-run mode
# It produces the below environment variables:
# - VERSION
# - DRA_COMMAND
# - DRA_BRANCH
dra_process_other_branches() {
  ## Read current version without the qualifier
  VERSION=$(make get-version-only)
  DRA_BRANCH="$BUILDKITE_BRANCH"
  if [[ $BUILDKITE_BRANCH =~ "feature/" ]]; then
    buildkite-agent annotate "${BUILDKITE_BRANCH} will list DRA artifacts. Feature branches are not supported. Look for the supported branches in ${BRANCHES_URL}" --style 'info' --context 'ctx-info'
    DRA_COMMAND=list

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
      fi
    fi
  fi
  if [ "$BUILDKITE_PULL_REQUEST" == "false" ]; then
    buildkite-agent annotate "Pull Requests are not supported by DRA. Look for the supported branches in ${BRANCHES_URL}" --style 'info' --context 'ctx-info-pr'
    DRA_COMMAND=list
    DRA_BRANCH=main
  fi
  export DRA_BRANCH DRA_COMMAND VERSION
}

# Create Buildkite annotation similarly done in Beats:
# https://github.com/elastic/beats/blob/90f9e8f6e48e76a83331f64f6c8c633ae6b31661/.buildkite/scripts/dra.sh#L74-L81
create_annotation_dra_summary() {
  local command=$1
  local workflow=$2
  local output=$3
  if [[ "$command" == "collect" ]]; then
    # extract the summary URL from a release manager output line like:
    # Report summary-18.22.0.html can be found at https://artifacts-staging.elastic.co/apm-server/18.22.0-ABCDEFGH/summary-18.22.0.html
    SUMMARY_URL=$(grep -E '^Report summary-.* can be found at ' "$output" | grep -oP 'https://\S+' | awk '{print $1}')
    rm "$output"

    # and make it easily clickable as a Builkite annotation
    printf "**${workflow} summary link:** [${SUMMARY_URL}](${SUMMARY_URL})\n" | buildkite-agent annotate --style=success --append
  fi
}
