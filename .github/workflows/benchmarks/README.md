# Benchmark Scripts

This directory contains utility scripts for users to run locally to kickstart and analyze APM Server benchmark workflows in different configurations.

The scripts here are used to generate the docs at https://www.elastic.co/docs/solutions/observability/apm/transaction-sampling#_tail_based_sampling_performance_and_requirements

## Workflow

1. **Run benchmarks**: Execute `benchmark-tbs.sh` to trigger multiple benchmark workflow runs on GitHub and save run IDs to `${BENCH_BRANCH}.txt`
2. **Wait for completion**: Monitor the workflow runs on GitHub Actions until all benchmarks finish
3. **Download results**: Run `download-benchmarks.sh` to fetch all benchmark artifacts using the saved run IDs
4. **Analyze results**: Run `analyze-benchmarks.sh` to generate benchstat analysis for each result
5. **Summarize for documentation**: Create a summary table from the benchstat results for the documentation (can be done manually or using an LLM to extract geomean values from each `benchstat.txt` file)

```bash
# Run benchmarks on the tbs-arm-bench-92 branch
BENCH_BRANCH=tbs-arm-bench-92 ./benchmark-tbs.sh

# Wait for workflows to complete on GitHub Actions

# Download all benchmark results using the saved run IDs
BENCH_BRANCH=tbs-arm-bench-92 ./download-benchmarks.sh

# Analyze the results
./analyze-benchmarks.sh
```

## benchmark-tbs.sh

Triggers multiple Tail-Based Sampling (TBS) benchmark workflows on GitHub Actions with different ARM instance configurations.

### Usage

```bash
BENCH_BRANCH=tbs-arm-bench-92 ./benchmark-tbs.sh
```

### Environment Variables

- `BENCH_BRANCH` (required): The git branch/ref to run the benchmarks against

### Description

This script triggers 9 benchmark workflow runs with different configurations:
- Tests 8GB, 16GB, and 32GB instance profiles
- Compares TBS disabled vs enabled configurations
- Tests both EBS gp3 volumes and local NVMe SSD storage
- Uses ARM-based EC2 instances (c6gd series)

The script runs configurations sequentially with 2-second delays between each and automatically saves each workflow run ID to `${BENCH_BRANCH}.txt` for later use with `download-benchmarks.sh`.

## download-benchmarks.sh

Downloads benchmark result artifacts from completed GitHub workflow runs.

### Usage

```bash
# Download all benchmark results (after running benchmark-tbs.sh)
BENCH_BRANCH=tbs-arm-bench-92 ./download-benchmarks.sh
```

### Environment Variables

- `BENCH_BRANCH` (required): The git branch/ref name, used to locate the file containing workflow run IDs

### Input

Reads workflow run IDs from `${BENCH_BRANCH}.txt` file (one per line). Each benchmark result is downloaded to a numbered directory (`benchmark-result-1`, `benchmark-result-2`, etc.).

## analyze-benchmarks.sh

Generates benchstat analysis for each downloaded benchmark result.

### Usage

```bash
./analyze-benchmarks.sh
# Creates benchmark-result-1/benchstat.txt, benchmark-result-2/benchstat.txt, etc.
```

### Description

For each `benchmark-result-*/benchmark-result.txt` file, runs `benchstat` and saves the output to `benchstat.txt` in the same directory.

### Requirements

- `benchstat` must be installed (`go install golang.org/x/perf/cmd/benchstat@latest`)
