#!/usr/bin/env bash
set -euox pipefail

export PLATFORMS='linux/amd64'
export TYPE='docker'

make release-manager-snapshot
