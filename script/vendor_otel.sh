#!/bin/sh

set -xe

go mod edit -dropreplace go.opentelemetry.io/collector
go mod download go.opentelemetry.io/collector

REPO_ROOT=$(go list -m -f {{.Dir}} github.com/elastic/apm-server)
MIXIN_DIR=$REPO_ROOT/internal/.otel_collector_mixin
TARGET_DIR=$REPO_ROOT/internal/otel_collector
MODULE_DIR=$(go list -m -f {{.Dir}} go.opentelemetry.io/collector)

rm -fr $TARGET_DIR
mkdir $TARGET_DIR
rsync -cr --no-perms --no-group --chmod=ugo=rwX --delete $MODULE_DIR/* $TARGET_DIR
rsync -cr --no-perms --no-group --chmod=ugo=rwX $MIXIN_DIR/* $TARGET_DIR

go mod edit -replace go.opentelemetry.io/collector=./internal/otel_collector
