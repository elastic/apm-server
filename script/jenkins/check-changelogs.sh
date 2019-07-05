#!/usr/bin/env bash
set -exuo pipefail

docker run --rm \
          -v "$PWD":/usr/src/myapp \
          -w /usr/src/myapp python:3.6.8-jessie \
          bash -c 'pip install requests; make check-changelogs'
