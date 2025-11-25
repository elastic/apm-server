#!/bin/sh

# This script is used for starting the workflows behind the documentation
# https://www.elastic.co/docs/solutions/observability/apm/transaction-sampling#_tail_based_sampling_performance_and_requirements

# Validate BENCH_BRANCH environment variable
if [ -z "$BENCH_BRANCH" ]; then
  echo "Error: BENCH_BRANCH environment variable is not set or is empty. Set it to the branch on which the workflow should run."
  exit 1
fi

gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=false -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/8GB_ARM-x1zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/8GB_ARM-x1zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/8GB_NVMe_ARM-x1zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m

sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=false -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/16GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/16GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/16GB_NVMe_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m

sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=false -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/32GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/32GB_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
sleep 2
gh workflow run benchmarks.yml --ref "$BENCH_BRANCH" -f runStandalone=true -f enableTailSampling=true -f tailSamplingStorageLimit=0 -f tailSamplingSampleRate=0.1 -f profile=system-profiles/32GB_NVMe_ARM-x2zone.tfvars -f pgoExport=false -f benchmarkAgents=1024 -f benchmarkRun=BenchmarkTraces -f warmupTime=5m
