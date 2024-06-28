#!/bin/sh

dirs=$(find . -maxdepth 1 -type d \! \( -name '.*' -or -name build \))
exec go run -modfile=tools/go.mod golang.org/x/tools/cmd/goimports $GOIMPORTSFLAGS -local github.com/elastic $dirs
