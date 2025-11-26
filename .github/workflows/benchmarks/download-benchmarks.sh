#!/bin/bash

# Validate BENCH_BRANCH environment variable
if [ -z "$BENCH_BRANCH" ]; then
  echo "Error: BENCH_BRANCH environment variable is not set or is empty"
  exit 1
fi

# Read workflow run IDs from ${BENCH_BRANCH}.txt
if [ ! -f "${BENCH_BRANCH}.txt" ]; then
  echo "Error: File ${BENCH_BRANCH}.txt not found"
  exit 1
fi

mapfile -t run_ids < "${BENCH_BRANCH}.txt"

# Download each benchmark result and rename
for i in "${!run_ids[@]}"; do
  run_id="${run_ids[$i]}"
  result_num=$((i + 1))
  
  echo "Downloading run $run_id as benchmark-result-$result_num..."
  gh run download "$run_id" -n benchmark-result -D "benchmark-result-$result_num"
done

echo "Done!"
