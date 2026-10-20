# trigger-obs-aw-status

Forwards failed Buildkite commit status events to the Observability Agentic Workflow status handler in the external `elastic/oblt-aw` repository.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/trigger-obs-aw-status.yml`](../trigger-obs-aw-status.yml) |
| Trigger | `status` |

## Purpose

This file is the repository-side client for the Observability Agentic Workflow status handling. It runs only for failed commit statuses whose context contains `buildkite`, then calls `elastic/oblt-aw/.github/workflows/obs-aw-event-status.yml@v0`. It passes a Buildkite API token to the called workflow so it can read Buildkite information. The file is owned by `@elastic/observablt-ci` according to the CODEOWNERS entry for `trigger-obs-aw-*` files.

## When it runs

- On `status` events where the commit status state is `failure` and the status context contains `buildkite`.
- All other status events are skipped by the job `if` condition.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `run-obs-aw-status` | Status event with state `failure` and a context containing `buildkite` | None declared | Calls `elastic/oblt-aw/.github/workflows/obs-aw-event-status.yml@v0` with the job permissions below and passes `BUILDKITE_API_TOKEN` from the `BUILDKITE_LOGS_API_TOKEN` secret. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Status event payload. No `with` inputs declared. |
| Outputs | Not declared in this file. |
| Permissions | Workflow level: `contents: read`. Job level: `actions: read`, `contents: read`, `copilot-requests: write`, `id-token: write`, `issues: write`, `pull-requests: write`. |
| Secrets | `BUILDKITE_API_TOKEN` is passed to the called workflow, sourced from the repository secret `BUILDKITE_LOGS_API_TOKEN`. Names only; values are not recorded. |

## Operational notes

- The called workflow is pinned to the `v0` ref of the external `elastic/oblt-aw` repository. Its behavior and any other secrets it requires are not verifiable from this repository (source gap).
- The `uses:` line carries a `ratchet:exclude` marker. The tool that reads this marker is not checked in here.

## Sources

- Workflow: [`.github/workflows/trigger-obs-aw-status.yml`](../trigger-obs-aw-status.yml)
- Ownership: [`.github/CODEOWNERS`](../../CODEOWNERS)
- Source gap: `elastic/oblt-aw/.github/workflows/obs-aw-event-status.yml@v0` is external and is not listed as a source.
