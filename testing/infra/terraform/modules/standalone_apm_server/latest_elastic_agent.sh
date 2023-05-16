#!/bin/bash

set -eo pipefail

LATEST_STACK=$(curl -s "https://artifacts-api.elastic.co/v1/versions" | jq -r '.versions[-1]')
LATEST_BUILD=$(curl -s "https://artifacts-api.elastic.co/v1/versions/${LATEST_STACK}/builds/" | jq -r '.builds[0]')

curl -s "https://artifacts-api.elastic.co/v1/versions/${LATEST_STACK}/builds/${LATEST_BUILD}/projects/elastic-agent" | jq -r ".project.packages | {deb: .\"elastic-agent-${LATEST_STACK}-amd64.deb\".url, rpm: .\"elastic-agent-${LATEST_STACK}-x86_64.rpm\".url }"
