#!/usr/bin/env bash
set -exuo pipefail

docker run --rm \
          -v "$PWD":/usr/src/myapp \
          -w /usr/src/myapp python:3.6.8-alpine \
          sh -c 'pip3 install requests; python3 script/check_changelogs.py'
