#!/usr/bin/env bash

set -euo pipefail

BEATS_BRANCH=$(grep "BEATS_VERSION?=" Makefile | awk 'awk BEGIN { FS = "=" } ; { print $2 }')

curl "https://api.github.com/repos/reakaleek/beats/commits?sha=${BEATS_BRANCH}" | jq --raw-output '.[0].sha' | xargs | cut -c 1-12
