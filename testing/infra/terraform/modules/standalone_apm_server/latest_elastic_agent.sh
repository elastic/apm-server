#!/bin/bash

set -eo pipefail

VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
  LATEST_VERSION_INFO=$(curl -s "https://snapshots.elastic.co/latest/master.json")
else
  LATEST_VERSION_INFO=$(curl -s "https://snapshots.elastic.co/latest/${VERSION}.json")
fi

# change snapshot version
VERSION=$(echo $LATEST_VERSION_INFO | jq -r '.version')

MANIFEST_URL=$(echo $LATEST_VERSION_INFO | jq -r '.manifest_url')

# get the download urls
curl -s "$MANIFEST_URL" | jq -r ".projects.\"elastic-agent-package\".packages | {deb_amd: .\"elastic-agent-${VERSION}-amd64.deb\".url, deb_arm: .\"elastic-agent-${VERSION}-arm64.deb\".url, rpm_amd: .\"elastic-agent-${VERSION}-x86_64.rpm\".url, rpm_arm: .\"elastic-agent-${VERSION}-aarch64.rpm\".url }"
