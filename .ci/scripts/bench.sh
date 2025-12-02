#!/usr/bin/env bash
set -euxo pipefail

BENCH_COUNT=5 make bench | tee bench.out
