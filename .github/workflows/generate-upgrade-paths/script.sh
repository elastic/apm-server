#!/usr/bin/env bash

set -euo pipefail

cd integrationservertest

SNAPSHOT_UPGRADE_PATHS=$(go run ./scripts/genpath --target='pro' --snapshots=true)
BC_UPGRADE_PATHS=$(go run ./scripts/genpath --target='pro' --snapshots=false)

echo "snapshot_upgrade_paths=${SNAPSHOT_UPGRADE_PATHS}" >> "${GITHUB_OUTPUT}"
echo "bc_upgrade_paths=${BC_UPGRADE_PATHS}" >> "${GITHUB_OUTPUT}"