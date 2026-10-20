# benchmarks-matrix

Runs the [benchmarks](./benchmarks.yml.md) workflow once for each system profile in a JSON list supplied at dispatch time.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`benchmarks-matrix.yml`](../benchmarks-matrix.yml) |
| Triggers | `workflow_dispatch` only |
| Concurrency | None |
| Token permissions | `contents: read` (workflow level). The `bench-matrix` job uses `contents: write` and `id-token: write`. |
| Secrets | Inherited from the caller (`secrets: inherit`). See [benchmarks](./benchmarks.yml.md). |
| Runner | Set by the called workflow (`ubuntu-latest`) |

## Purpose

Runs the same benchmark configuration against several system profiles in one dispatch.

## When it runs

Only on demand. There is no schedule.

## Inputs

| Input | Required | Effect |
|-------|----------|--------|
| `profilesJSON` | Yes | JSON list of system profiles, for example `["system-profiles/8GBx1zone.tfvars"]`. Drives the matrix. |
| `runStandalone`, `enableTailSampling`, `tailSamplingStorageLimit`, `runOnStable`, `benchmarkAgents`, `benchmarkRun`, `warmupTime` | No | Passed unchanged to every matrix run. |

## Jobs and steps

| Job | Steps |
|-----|-------|
| `bench-matrix` | Runs a matrix over `profilesJSON`. Each entry calls [`benchmarks.yml`](../benchmarks.yml) with the matching `profile` and the shared inputs. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Outputs | One set of benchmark artifacts and reports per profile, from the called workflow. |
| Permissions | Job-level `contents: write` and `id-token: write`, passed to the called workflow. |
| Secrets | Inherited. The called workflow needs the allowlist described in [benchmarks](./benchmarks.yml.md). |

## Operational notes

- Matrix runs are independent. Each one creates its own benchmark environment and destroys it in its own cleanup step.
- The matrix input has no default. An invalid JSON value fails the run before any benchmark starts.

## Sources

- [`.github/workflows/benchmarks-matrix.yml`](../benchmarks-matrix.yml)
- [`.github/workflows/benchmarks.yml`](../benchmarks.yml)
