#!/usr/bin/env bash
set -euxo pipefail

make bench | tee bench.out
