#!/usr/bin/env bash
##
##  Downloads build artifacts from Buildkite storage and stages them into
##  artifacts/ for the elastic/dra-prep plugin.
##
##  The package pipeline builds both snapshot and staging via a matrix in the
##  same build, so both artifact sets land in the artifact store. Filter by the
##  SNAPSHOT filename convention so each DRA prep job only stages its own set.
##

set -euo pipefail

WORKFLOW="${DRA_WORKFLOW:?DRA_WORKFLOW is required}"

echo "--- Restoring Artifacts"
buildkite-agent artifact download "build/distributions/**/*" .
buildkite-agent artifact download "build/dependencies*.csv" .

echo "--- Prepare ${WORKFLOW} artifacts"
mkdir -p artifacts

if [[ "${WORKFLOW}" == "snapshot" ]]; then
  find build/distributions -maxdepth 1 -type f -name "*-SNAPSHOT-*" -exec cp {} artifacts/ \;
  cp build/dependencies-*-SNAPSHOT.csv artifacts/ 2>/dev/null || true
else
  find build/distributions -maxdepth 1 -type f ! -name "*-SNAPSHOT-*" -exec cp {} artifacts/ \;
  find build -maxdepth 1 -type f -name "dependencies-*.csv" ! -name "*-SNAPSHOT.csv" -exec cp {} artifacts/ \;
fi

if ! ls artifacts/* >/dev/null 2>&1; then
  echo "ERROR: no ${WORKFLOW} artifacts found in artifacts/" >&2
  exit 1
fi

echo "Staged artifacts:"
ls -1 artifacts/
