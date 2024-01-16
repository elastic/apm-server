#!/usr/bin/env bash
#
# This script updates .go-version, documentation, and build files to use the
# most recent patch version for the major.minor Go version defined in go.mod.
set -e

SDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $SDIR/..

# Use gimme to find the latest patch release for the major.minor
# Go version defined in go.mod.
GOMOD_VERSION=$(grep '^go' go.mod | cut -d' ' -f2 | cut -d. -f1-2)

# TODO(axw) arrange for Go 1.21+ to always be available, and stop using gimme.
eval $(script/gimme/gimme $GOMOD_VERSION.x)
GOVERSION=$(go env GOVERSION | sed 's/^go//')

find -name go.mod -execdir go get toolchain@$GOVERSION \;
echo $GOVERSION > .go-version
sed -i "s/golang:[[:digit:]]\+\(\.[[:digit:]]\+\)\{1,2\}/golang:$GOVERSION/" packaging/docker/Dockerfile
sed -i "s/\(\[Go\]\[golang-download\]\) [[:digit:]].*/\1 $GOMOD_VERSION.x/" README.md
sed -i "s/toolchain go[[:digit:]]\+\(\.[[:digit:]]\+\)\{1,2\}/toolchain go$GOVERSION/" tools/go.mod
sed -i "s/toolchain go[[:digit:]]\+\(\.[[:digit:]]\+\)\{1,2\}/toolchain go$GOVERSION/" internal/glog/go.mod
sed -i "s/toolchain go[[:digit:]]\+\(\.[[:digit:]]\+\)\{1,2\}/toolchain go$GOVERSION/" go.mod
sed -i "s/toolchain go[[:digit:]]\+\(\.[[:digit:]]\+\)\{1,2\}/toolchain go$GOVERSION/" systemtest/go.mod
sed -i "s/toolchain go[[:digit:]]\+\(\.[[:digit:]]\+\)\{1,2\}/toolchain go$GOVERSION/" cmd/intake-receiver/go.mod
