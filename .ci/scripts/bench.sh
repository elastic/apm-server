#!/usr/bin/env bash
set -euxo pipefail

BENCH_COUNT=6 make bench | tee bench.out
