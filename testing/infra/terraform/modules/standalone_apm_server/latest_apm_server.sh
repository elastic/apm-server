#!/bin/bash

set -eo pipefail

VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
  LATEST_VERSION_INFO=$(curl -s "https://snapshots.elastic.co/latest/master.json")
else
  LATEST_VERSION_INFO=$(curl -s "https://snapshots.elastic.co/latest/${VERSION}.json")
fi

# change to the snapshot version
VERSION=$(echo $LATEST_VERSION_INFO | jq -r '.version')

MANIFEST_URL=$(echo $LATEST_VERSION_INFO | jq -r '.manifest_url')

# get the download urls
curl -s "$MANIFEST_URL" | jq -r ".projects.\"apm-server\".packages | {deb_amd: .\"apm-server-${VERSION}-amd64.deb\".url, deb_arm: .\"apm-server-${VERSION}-arm64.deb\".url, rpm_amd: .\"apm-server-${VERSION}-x86_64.rpm\".url, rpm_arm: .\"apm-server-${VERSION}-aarch64.rpm\".url }"

