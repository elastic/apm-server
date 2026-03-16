#!/bin/bash

set -euo pipefail

PGO_BRANCH="update-pgo-$(date +%s)"
cd "$WORKSPACE_PATH"
git fetch origin main
git checkout main
git checkout -b "$PGO_BRANCH"
mv "$PROFILE_PATH" x-pack/apm-server/default.pgo
git add x-pack/apm-server/default.pgo

# Skip PR creation when the generated profile doesn't change.
if git diff --cached --quiet; then
  echo "No changes in x-pack/apm-server/default.pgo, skipping PR creation."
  exit 0
fi

git commit -m "PGO: Update default.pgo from benchmarks $WORKFLOW."
git push -u origin "$PGO_BRANCH"
PR_URL="$(gh pr create -B main -H "$PGO_BRANCH" -t "PGO: Update default.pgo" -b "Update default.pgo CPU profile from the benchmarks [workflow]($WORKFLOW)." -R elastic/apm-server)"
gh pr merge --auto "$PR_URL"