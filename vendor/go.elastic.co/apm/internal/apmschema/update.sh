#!/usr/bin/env bash

set -ex

BRANCH=master

FILES=( \
    "errors/common_error.json" \
    "errors/v2_error.json" \
    "sourcemaps/payload.json" \
    "spans/common_span.json" \
    "spans/v2_span.json" \
    "transactions/mark.json" \
    "transactions/common_transaction.json" \
    "transactions/v2_transaction.json" \
    "metricsets/common_metricset.json" \
    "metricsets/v2_metricset.json" \
    "metricsets/sample.json" \
    "context.json" \
    "metadata.json" \
    "process.json" \
    "request.json" \
    "service.json" \
    "stacktrace_frame.json" \
    "system.json" \
    "tags.json" \
    "timestamp_epoch.json" \
    "user.json" \
)

mkdir -p jsonschema/errors jsonschema/transactions jsonschema/sourcemaps jsonschema/spans jsonschema/metricsets

for i in "${FILES[@]}"; do
  o=jsonschema/$i
  curl -sf https://raw.githubusercontent.com/elastic/apm-server/${BRANCH}/docs/spec/${i} --compressed -o $o
done
