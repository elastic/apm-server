#!/usr/bin/env bash
set -xeuo pipefail

make fmt

if [ -n "$(git status --porcelain)" ]; then
  echo "Lint detected files not following project's format. Please run 'make fmt' locally to fix this error"
  exit 1
fi
