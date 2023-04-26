#!/bin/sh

dirs=$(find . -maxdepth 1 -type d \! \( -name '.*' -or -name build -or -name internal \))
exec goimports $GOIMPORTSFLAGS -local github.com/elastic *.go $dirs
