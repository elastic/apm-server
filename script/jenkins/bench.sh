#!/usr/bin/env bash
set -euxo pipefail

go test -benchmem -run=XXX -benchtime=100ms -bench='.*' ./... | tee bench.out
