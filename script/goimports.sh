#!/bin/sh

dirs=$(find . -maxdepth 1 -type d \! \( -name '.*' -or -name build \))
exec go tool golang.org/x/tools/cmd/goimports $GOIMPORTSFLAGS -local github.com/elastic $dirs
