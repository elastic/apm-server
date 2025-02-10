#!/bin/bash

set -eo pipefail

VERSION=${1}
if [[ -z ${VERSION} ]] || [[ "${VERSION}" == "latest" ]]; then
    VERSION=$(curl -s "https://artifacts-api.elastic.co/v1/versions" | jq -r '.versions[-1]')
fi
LATEST_BUILD=$(curl -s "https://artifacts-api.elastic.co/v1/versions/${VERSION}/builds/" | jq -r '.builds[0]')

curl -s "https://artifacts-api.elastic.co/v1/versions/${VERSION}/builds/${LATEST_BUILD}/projects/apm-server" | jq -r ".project.packages | {deb_amd: .\"apm-server-${VERSION}-amd64.deb\".url, deb_arm: .\"apm-server-${VERSION}-arm64.deb\".url, rpm_amd: .\"apm-server-${VERSION}-x86_64.rpm\".url, rpm_arm: .\"apm-server-${VERSION}-aarch64.rpm\".url }"
