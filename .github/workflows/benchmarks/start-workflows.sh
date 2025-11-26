#!/bin/sh

# Validate BENCH_BRANCH environment variable
if [ -z "$BENCH_BRANCH" ]; then
  echo "Error: BENCH_BRANCH environment variable is not set or is empty. Set it to the branch on which the workflow should run."
  exit 1
fi

# 8GB, TBS disabled
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=false -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/8GB_ARM-x1zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"
sleep 2
# 8GB, TBS enabled, gp3 3000IOPS
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/8GB_ARM-x1zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"
sleep 2
# 8GB, TBS enabled, NVMe
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/8GB_NVMe_ARM-x1zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"

sleep 2
# 16GB, TBS disabled
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=false -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/16GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"
sleep 2
# 16GB, TBS enabled, gp3 3000IOPS
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/16GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"
sleep 2
# 16GB, TBS enabled, NVMe
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/16GB_NVMe_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"

sleep 2
# 32GB, TBS disabled
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=false -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/32GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"
sleep 2
# 32GB, TBS enabled, gp3 3000IOPS
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/32GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"
sleep 2
# 32GB, TBS enabled, NVMe
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/32GB_NVMe_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
gh run list --workflow=benchmarks.yml --branch "$BENCH_BRANCH" --json 'databaseId' -q '.[0].databaseId' >> "${BENCH_BRANCH}.txt"
