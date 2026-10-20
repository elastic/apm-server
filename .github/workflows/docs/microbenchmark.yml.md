# microbenchmark

Triggers the `apm-agent-microbenchmark` Buildkite pipeline on pushes to the main and release branches. That pipeline runs `.ci/scripts/bench.sh` against this repository.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`microbenchmark.yml`](../microbenchmark.yml) |
| Triggers | `workflow_dispatch`; `push` to `main`, `8.*`, and `9.*` excluding Markdown and AsciiDoc changes |
| Concurrency | None |
| Token permissions | `contents: read` (workflow level) |
| Secrets | `BUILDKITE_TOKEN` |
| Runner | `ubuntu-latest`, 5-minute timeout |

## Purpose

Starts the Go microbenchmark run on Buildkite and gives it the commit and branch. The GitHub job only triggers the run. It does not run benchmarks itself.

## When it runs

- On demand.
- On push to `main`, `8.*`, or `9.*` when a change touches a file that is not Markdown or AsciiDoc.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `microbenchmark` | Runs `elastic/oblt-actions/buildkite/run@v1` with the `BUILDKITE_TOKEN` secret. It passes `script=.ci/scripts/bench.sh`, `repo=apm-server`, the commit SHA, and the branch name to the `apm-agent-microbenchmark` pipeline. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | None on the GitHub side. The pipeline receives the environment variables above. |
| Outputs | A Buildkite build. Results are indexed and diffed inside the Buildkite job. |
| Permissions | Read-only for the GitHub token. |
| Secrets | `BUILDKITE_TOKEN`, name only. |

## Operational notes

- [`bench.sh`](../../../.ci/scripts/bench.sh) runs `make bench` with a benchmark count of six. On CI it uploads the result, indexes it to Elasticsearch, downloads the last successful build's result, and uploads a `benchstat` diff. Those steps run in the Buildkite pipeline, not in this workflow.
- `bench.sh` exits with a message when no matching artifact is found in the last successful build, so the comparison step does not run in that case.
- The Buildkite pipeline definition for `apm-agent-microbenchmark` lives in another repository and was not inspected. This is a source gap.

## Sources

- [`.github/workflows/microbenchmark.yml`](../microbenchmark.yml)
- [`.ci/scripts/bench.sh`](../../../.ci/scripts/bench.sh)
- Source gap: the `apm-agent-microbenchmark` pipeline definition is outside this repository and was not inspected.
