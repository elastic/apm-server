#!/usr/bin/env bash
#
# This script updates .go-version, documentation, and build files to use the
# most recent patch version for the major.minor Go version defined in go.mod.
set -e

SDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $SDIR/..

# Go version defined in go.mod.
MAJOR_MINOR_VERSION=$(grep '^go' go.mod | cut -d' ' -f2 | cut -d. -f1-2)

find ./ -type f -name "go.mod" -execdir go get go@$MAJOR_MINOR_VERSION \; -execdir go get toolchain@none \;

GO_VERSION=$(grep '^go' go.mod | cut -d' ' -f2)
echo $GO_VERSION > .go-version
