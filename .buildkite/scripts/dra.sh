#!/usr/bin/env bash
##
##  It relies on the .buildkite/hooks/pre-command so the Vault and other tooling
##  are prepared automatically by buildkite.
##
##  It uploads DRA prep pipeline steps and, on active release branches, also
##  triggers unified-release-dra-processing. On non-active branches (feature branches)
##  the plugin runs in dry-run mode (upload: false) so contributors can validate 
##  classification and manifest generation without publishing to GCS.
##

set -eo pipefail

# Either staging or snapshot
TYPE="$1"

# NOTE: load the shared functions
# shellcheck disable=SC1091
source .buildkite/scripts/utils.sh

# by default it uses the buildkite branch
DRA_BRANCH="$BUILDKITE_BRANCH"
VERSION=$(make get-version-only)
BRANCHES_URL=https://storage.googleapis.com/artifacts-api/snapshots/branches.json
curl -fsS "${BRANCHES_URL}" > active-branches.json
# Publish to DRA GCS only on active release branches. Non-active branches run
# the plugin in dry-run mode (upload: false) so PRs and feature branches can
# still validate their packaging without publishing.
DRA_UPLOAD=true
# if ! grep -Fq "\"$BUILDKITE_BRANCH\"" active-branches.json ; then
#   DRA_UPLOAD=false
# fi

echo "--- :arrow_right: DRA context"
echo "BUILDKITE_BRANCH=$BUILDKITE_BRANCH"
echo "BUILDKITE_COMMIT=$BUILDKITE_COMMIT"
echo "VERSION=$VERSION"
echo "DRA_UPLOAD=$DRA_UPLOAD"
echo "Supported branches:"
cat active-branches.json

dra() {
  local workflow=$1
  local qualifier=${2:-""}
  local stack_version="${VERSION}"
  # For pre-release staging builds (alpha, rc, ...) the qualifier must be part
  # of stack_version so the plugin publishes under e.g. 9.0.0-alpha1 rather
  # than 9.0.0. Snapshot builds never carry a qualifier.
  if [[ -n "${qualifier}" ]]; then
    stack_version="${VERSION}-${qualifier}"
  fi
  # On dry-runs, skip both the trigger (nothing was uploaded for processing) and
  # the summary annotator (the summary URL would 404 since upload was skipped).
  local trigger_step=""
  local annotate_step=""
  if [[ "${DRA_UPLOAD}" == "true" ]]; then
    trigger_step=$(cat <<TRIG

  - label: ":pipeline: DRA processing for apm-server (${workflow})"
    trigger: "unified-release-dra-processing"
    depends_on: "dra-prep-${workflow}"
    build:
      env:
        DRA_PRODUCT_ID: "apm-server"
        DRA_STACK_VERSION: "${stack_version}"
        DRA_WORKFLOW: "${workflow}"
TRIG
)
    annotate_step=$(cat <<ANN

  - label: ":memo: Annotate DRA summary (${workflow})"
    key: "dra-annotate-${workflow}"
    depends_on: "dra-prep-${workflow}"
    command: ".buildkite/scripts/dra-annotate.sh ${workflow}"
    agents:
      provider: "gcp"
      image: "${IMAGE_UBUNTU_X86_64}"
    timeout_in_minutes: 5
ANN
)
  fi

  echo "--- Generating DRA sub-pipeline for $workflow (upload=${DRA_UPLOAD})"
  cat <<PIPELINE | buildkite-agent pipeline upload
steps:
  - label: ":package: DRA Prep (${workflow})"
    key: "dra-prep-${workflow}"
    command: ".buildkite/scripts/stage-dra-artifacts.sh"
    env:
      DRA_WORKFLOW: "${workflow}"
    agents:
      provider: "gcp"
      image: "${IMAGE_UBUNTU_X86_64}"
      machineType: "c2-standard-16"
    timeout_in_minutes: 30
    artifact_paths:
      - "artifacts/dra/apm-server/*/manifest-*.json"
    plugins:
      - elastic/dra-prep#v0.1.5:
          product_id: "apm-server"
          stack_version: "${stack_version}"
          workflow: "${workflow}"
          upload: ${DRA_UPLOAD}
${annotate_step}
${trigger_step}
PIPELINE
}

if [[ "${TYPE}" == "staging" ]]; then
  qualifier=$(fetch_elastic_qualifier "$DRA_BRANCH")
  # TODO: main and 8.x are not needed to run the DRA for staging
  #       but main is needed until we do alpha1 releases of 9.0.0
  if [[ "${DRA_BRANCH}" != "8.x" ]]; then
    dra "${TYPE}" "${qualifier}"
  fi
fi

if [[ "${TYPE}" == "snapshot" ]]; then
  # NOTE: qualifier is not needed for snapshots, let's unset it.
  dra "${TYPE}" ""
fi
