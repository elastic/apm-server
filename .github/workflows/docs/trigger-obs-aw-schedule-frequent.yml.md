# trigger-obs-aw-schedule-frequent

Runs the Observability Agentic Workflow scheduled handler in its `frequent` profile, through the external `elastic/oblt-aw` repository.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/trigger-obs-aw-schedule-frequent.yml`](../trigger-obs-aw-schedule-frequent.yml) |
| Trigger | Runs on a schedule (high cadence); `workflow_dispatch` |

## Purpose

This file is the high-cadence lane of the Observability Agentic Workflow scheduled client. It calls `elastic/oblt-aw/.github/workflows/obs-aw-event-schedule.yml@v0` with `schedule-profile: frequent`. The source comment states that the daily-only routes in the called workflow are kept off this lane by the profile. The daily lane is in [`trigger-obs-aw-schedule-daily.yml`](./trigger-obs-aw-schedule-daily.yml.md). The file is owned by `@elastic/observablt-ci` according to the CODEOWNERS entry for `trigger-obs-aw-*` files.

## When it runs

- Runs on a schedule at a high cadence, and can be started manually with `workflow_dispatch`.
- The file declares no job condition.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `run-obs-aw-schedule-frequent` | On the scheduled high-cadence run or manual dispatch | None declared | Calls `elastic/oblt-aw/.github/workflows/obs-aw-event-schedule.yml@v0` with `schedule-profile: frequent`. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | `schedule-profile: frequent`. |
| Outputs | Not declared in this file. |
| Permissions | Workflow level: `contents: read`. Job level: `actions: read`, `contents: write`, `copilot-requests: write`, `id-token: write`, `issues: write`, `pull-requests: write`. The source comment says these are the union of the permissions of every job in the called workflow, because GitHub checks callers against all declared jobs. |
| Secrets | None passed explicitly. |

## Operational notes

- The source comment says the daily-only routes in the called workflow are skipped on this lane through their own conditions. The routes and conditions are defined in the external workflow and are not verifiable here (source gap).
- The called workflow is pinned to the `v0` ref of the external `elastic/oblt-aw` repository.
- The `uses:` line carries a `ratchet:exclude` marker. The tool that reads this marker is not checked in here.

## Sources

- Workflow: [`.github/workflows/trigger-obs-aw-schedule-frequent.yml`](../trigger-obs-aw-schedule-frequent.yml)
- Companion schedule lane: [`.github/workflows/trigger-obs-aw-schedule-daily.yml`](../trigger-obs-aw-schedule-daily.yml)
- Ownership: [`.github/CODEOWNERS`](../../CODEOWNERS)
- Source gap: `elastic/oblt-aw/.github/workflows/obs-aw-event-schedule.yml@v0` is external and is not listed as a source.
