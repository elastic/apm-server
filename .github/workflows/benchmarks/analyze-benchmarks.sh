#!/bin/bash

# Analyze each benchmark result
for dir in benchmark-result-*/; do
  if [ -f "$dir/benchmark-result.txt" ]; then
    echo "=== Analyzing $dir ==="
    benchstat "$dir/benchmark-result.txt" > "$dir/benchstat.txt"
    echo "Saved to $dir/benchstat.txt"
  fi
done
