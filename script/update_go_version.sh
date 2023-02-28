#!/usr/bin/env bash
#
# This script updates .go-version, documentation, and build files to use the
# most recent patch version for the major.minor Go version defined in go.mod.
set -e

SDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $SDIR/..

# Use gimme to find the latest patch release for the major.minor
# Go version defined in go.mod.
GOMOD_VERSION=$(grep '^go' go.mod | cut -d' ' -f2)
eval $(script/gimme/gimme $GOMOD_VERSION.x)
GOVERSION=$(go env GOVERSION | sed 's/^go//')

echo $GOVERSION > .go-version
sed -i "s/golang:[[:digit:]]\+\(\.[[:digit:]]\+\)\{1,2\}/golang:$GOVERSION/" packaging/docker/Dockerfile
sed -i "s/^:go-version:.*/:go-version: $GOVERSION/" docs/version.asciidoc
sed -i "s/\(\[Go\]\[golang-download\]\) [[:digit:]].*/\1 $GOMOD_VERSION.x/" README.md
