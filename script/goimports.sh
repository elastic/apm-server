#!/bin/sh

dirs=$(find . -maxdepth 1 -type d \! \( -name '.*' -or -name build \))
exec goimports $GOIMPORTSFLAGS -local github.com/elastic *.go $dirs
