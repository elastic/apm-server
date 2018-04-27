#!/usr/bin/env bash

set -e
rm -f coverage.txt

for pkg in $(go list ./...); do
    go test -coverprofile=profile.out -covermode=atomic $pkg
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
done
