#!/usr/bin/env bash
##
##  Downloads the DRA manifest from the dra-prep-${WORKFLOW} step,
##  extracts build_id and version, and annotates the build with a link to
##  the workflow's published summary.
##
##  dractl prep never writes a summary-*.html at its temp upload path (only
##  the manifest) — the rendered summary only exists once
##  unified-release-dra-processing publishes it to the final location, so
##  link there instead of the temp path.
##
##  Invoked from the generated DRA sub-pipeline. Kept as a standalone script
##  because Buildkite interpolates inline command:'s ${VAR} references at
##  job pickup, which would eat local variables set inside the command block.
##

set -euo pipefail

WORKFLOW="${1:?workflow required}"

buildkite-agent artifact download "artifacts/dra/apm-server/*/manifest-*.json" . --step "dra-prep-${WORKFLOW}"
manifest=$(find artifacts/dra/apm-server -name "manifest-*.json" | head -1)
prefix=$(jq -r '.prefix' "${manifest}")
build_id=$(jq -r '.build_id' "${manifest}")
version=$(jq -r '.version' "${manifest}")
url="https://artifacts-${WORKFLOW}.elastic.co/${prefix}/${build_id}/summary-${version}.html"

printf "**%s summary link:** [%s](%s)\n" "${WORKFLOW}" "${url}" "${url}" | buildkite-agent annotate --style=success --append
