# benchmarks

Runs the APM Server benchmark suite on a provisioned Elastic Cloud environment, indexes the results, publishes a Kibana PNG report and a CPU profile, and optionally opens a PR that updates the PGO profile. It runs on demand, on a daily schedule, and on a monthly PGO schedule. It is also called by [benchmarks-matrix](./benchmarks-matrix.yml.md).

## At a glance

| Item | Value |
|------|-------|
| Definition | [`benchmarks.yml`](../benchmarks.yml) |
| Triggers | `workflow_dispatch`; `workflow_call` (same inputs); `schedule`: daily regular run and monthly PGO run |
| Concurrency | None |
| Token permissions | `contents: read` (workflow level). The `benchmarks` job uses `contents: write` and `id-token: write`. |
| Secrets | `GOBENCH_PASSWORD`, `GOBENCH_USERNAME`, `GOBENCH_HOST`, `ELASTIC_DOCKER_REGISTRY`, `ELASTIC_DOCKER_USERNAME`, `ELASTIC_DOCKER_PASSWORD`, `KIBANA_BENCH_ENDPOINT`, `KIBANA_BENCH_USERNAME`, `KIBANA_BENCH_PASSWORD`, `KIBANA_BENCH_DASHBOARD`, `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`, `SLACK_BOT_TOKEN` |
| Runner | `ubuntu-latest` |

## Purpose

Measures APM Server performance against a benchmark environment and keeps the results and the PGO profile current.

## When it runs

- **On demand:** `workflow_dispatch`, or as a called workflow from the matrix workflow.
- **Daily:** the regular run. It excludes the tracing benchmarks (`BenchmarkTraces*`) and uses a shorter warm-up.
- **Monthly:** the PGO run. It also deploys the standalone server with Moxy and exports a PGO profile.

## Inputs

| Input | Default | Effect |
|-------|---------|--------|
| `runStandalone` | `false` | Benchmarks a standalone APM Server with Moxy. Forced on for the monthly PGO schedule. |
| `enableTailSampling` | `false` | Enables tail-based sampling. Storage limit and sample rate have their own inputs. |
| `tailSamplingStorageLimit` | `10GB` | Storage limit for tail-based sampling. |
| `tailSamplingSampleRate` | `0.1` | Blanket sample rate for tail-based sampling. |
| `profile` | `system-profiles/8GBx1zone.tfvars` | System profile passed to Terraform. |
| `runOnStable` | `false` | Benchmarks the latest stable version instead of a build from the commit. |
| `benchmarkAgents` | Makefile default | Number of benchmark agents. An empty value uses the autotuned run. |
| `benchmarkRun` | Makefile default | Regex that selects scenarios. |
| `warmupTime` | Set in the workflow | Warm-up time, for example `3s`, `5m`, or `2h`. |
| `pgoExport` | `false` | Opens a PR that updates `default.pgo`. Forced on for the monthly PGO schedule. |

## Jobs and steps

The workflow has one job, `benchmarks`, which runs in `testing/benchmark`.

| Phase | Steps |
|-------|-------|
| Set up | Checks out the repository, sets up Go from `go.mod`, computes a slugged branch name and timestamps, sets the warm-up time and scenario regex, and excludes tracing benchmarks on the daily run. |
| Access | Logs in to the Elastic container registry, authenticates to Google Cloud and AWS through `elastic/oblt-actions`, and loads `EC_API_KEY` from Secret Manager. |
| Infrastructure | Installs Terraform, runs `make init`, builds `apmbench`, optionally builds APM Server and Moxy, overrides the Docker committed version (not on the standalone or stable path), and runs `make apply`. |
| Benchmark | Runs `make run-benchmark-autotuned` when no agent count is set, or `make run-benchmark` otherwise. Dumps standalone logs on failure, then runs `make index-benchmark-results`. |
| Reports | Downloads a PNG from Kibana with [`download-png-from-kibana.sh`](../../../.ci/scripts/download-png-from-kibana.sh). Uploads the PNG as a workflow artifact and to AWS S3. Uploads the benchmark result as an artifact. |
| PGO | Copies the CPU profile and uploads it as an artifact. Creates a GitHub App token and configures git with GPG signing. When PGO export is on, runs [`push-pgo-pr.sh`](../../../.ci/scripts/push-pgo-pr.sh) to open the profile PR. |
| Cleanup | Re-loads `EC_API_KEY` and runs `make init destroy` even on failure. |
| Notify | On the scheduled runs only, posts a failure message or a success message with the report image to Slack. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Outputs | Artifacts: `kibana-png-report`, `benchmark-result`, `cpu-profile`. S3 object for the PNG. Slack messages on schedule. Optional PGO PR. |
| Permissions | Job-level `contents: write` and `id-token: write` for cloud auth and the PR. The GitHub App token has `contents` and `pull-requests` write. |
| Secrets | Names only, listed above. Secret access requires an allowlist in the external `oblt-infra` configuration. |

## Operational notes

- A source comment says the secrets are rotated daily, so a run that overlaps the rotation window can fail. The workflow loads `EC_API_KEY` both before and after the run.
- The Slack failure message on scheduled runs points to an internal runbook. The link is an external URL and is not checked here.
- The `TF_VAR_*` values and `GOBENCH_TAGS` come from the workflow `env`. Makefile targets and Terraform modules were not inspected in this pass. These are source gaps.
- The PNG script reads a fixed dashboard and time range from the script. It was read only in part, so its full retry and error handling is a source gap.
- The S3 upload step uses a fixed bucket in `us-east-1`.

## Sources

- [`.github/workflows/benchmarks.yml`](../benchmarks.yml)
- [`.github/workflows/benchmarks-matrix.yml`](../benchmarks-matrix.yml)
- [`.ci/scripts/download-png-from-kibana.sh`](../../../.ci/scripts/download-png-from-kibana.sh) (partly read)
- [`.ci/scripts/push-pgo-pr.sh`](../../../.ci/scripts/push-pgo-pr.sh)
- Source gaps: Makefile targets (`apmbench`, `apply`, `destroy`, `run-benchmark`, `run-benchmark-autotuned`, `index-benchmark-results`, `cp-cpuprof`, `docker-override-committed-version`) and the Terraform code under `testing/benchmark` were not inspected.
