#!/bin/bash

set -eo pipefail

PGO_BRANCH="update-pgo-$(date +%s)"
cd $WORKSPACE_PATH
git fetch origin main
git checkout main
git checkout -b $PGO_BRANCH
mv $PROFILE_PATH x-pack/apm-server/default.pgo
git add x-pack/apm-server/default.pgo
git commit -m "PGO: Update default.pgo from benchmarks $WORKFLOW."
git push -u origin $PGO_BRANCH
gh pr create -B main -H $PGO_BRANCH -t "PGO: Update default.pgo" -b "Update default.pgo CPU profile from the benchmarks [workflow]($WORKFLOW)." -R elastic/apm-server
gh pr merge --auto --delete-branch --squash $PGO_BRANCH