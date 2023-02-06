#!/usr/bin/env bash

set -euo pipefail

curl https://api.github.com/repos/elastic/beats/commits | jq --raw-output '.[0].sha' | xargs | cut -c 1-12
