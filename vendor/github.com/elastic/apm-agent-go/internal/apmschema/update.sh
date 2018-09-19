#!/usr/bin/env bash

set -ex

FILES=( \
    "errors/error.json" \
    "errors/payload.json" \
    "sourcemaps/payload.json" \
    "transactions/mark.json" \
    "transactions/payload.json" \
    "transactions/span.json" \
    "transactions/transaction.json" \
    "metrics/payload.json" \
    "metrics/metric.json" \
    "metrics/sample.json" \
    "context.json" \
    "process.json" \
    "request.json" \
    "service.json" \
    "stacktrace_frame.json" \
    "system.json" \
    "user.json" \
)

mkdir -p jsonschema/errors jsonschema/transactions jsonschema/sourcemaps jsonschema/metrics

for i in "${FILES[@]}"; do
  o=jsonschema/$i
  curl -sf https://raw.githubusercontent.com/elastic/apm-server/master/docs/spec/${i} --compressed -o $o
done
