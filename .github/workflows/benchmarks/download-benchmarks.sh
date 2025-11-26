#!/bin/bash

# Read workflow run IDs from stdin
mapfile -t run_ids

# Download each benchmark result and rename
for i in "${!run_ids[@]}"; do
  run_id="${run_ids[$i]}"
  result_num=$((i + 1))
  
  echo "Downloading run $run_id as benchmark-result-$result_num..."
  gh run download "$run_id" -n benchmark-result -D "benchmark-result-$result_num"
done

echo "Done!"
