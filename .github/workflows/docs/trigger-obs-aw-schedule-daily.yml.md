# trigger-obs-aw-schedule-daily

Runs the Observability Agentic Workflow scheduled handler in its `daily` profile, through the external `elastic/oblt-aw` repository.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/trigger-obs-aw-schedule-daily.yml`](../trigger-obs-aw-schedule-daily.yml) |
| Trigger | Runs on a schedule (daily); `workflow_dispatch` |

## Purpose

This file is the daily lane of the Observability Agentic Workflow scheduled client. It calls `elastic/oblt-aw/.github/workflows/obs-aw-event-schedule.yml@v0` with `schedule-profile: daily`. A second scheduled lane lives in [`trigger-obs-aw-schedule-frequent.yml`](./trigger-obs-aw-schedule-frequent.yml.md). The source comment explains that GitHub does not expose which cron fired when several expressions share one workflow file, which is why the lanes are split into two files. The file is owned by `@elastic/observablt-ci` according to the CODEOWNERS entry for `trigger-obs-aw-*` files.

## When it runs

- Runs on a schedule (daily), and can be started manually with `workflow_dispatch`.
- The file declares no job condition.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `run-obs-aw-schedule-daily` | On the scheduled daily run or manual dispatch | None declared | Calls `elastic/oblt-aw/.github/workflows/obs-aw-event-schedule.yml@v0` with `schedule-profile: daily`. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | `schedule-profile: daily`. |
| Outputs | Not declared in this file. |
| Permissions | Workflow level: `contents: read`. Job level: `actions: read`, `contents: write`, `copilot-requests: write`, `id-token: write`, `issues: write`, `pull-requests: write`. |
| Secrets | None passed explicitly. |

## Operational notes

- The source comments say the daily profile is where the daily-only routes run, and name autodoc and security routes as examples. The routes themselves are defined in the external workflow and are not verifiable here (source gap).
- The called workflow is pinned to the `v0` ref of the external `elastic/oblt-aw` repository.
- The `uses:` line carries a `ratchet:exclude` marker. The tool that reads this marker is not checked in here.

## Sources

- Workflow: [`.github/workflows/trigger-obs-aw-schedule-daily.yml`](../trigger-obs-aw-schedule-daily.yml)
- Companion schedule lane: [`.github/workflows/trigger-obs-aw-schedule-frequent.yml`](../trigger-obs-aw-schedule-frequent.yml)
- Ownership: [`.github/CODEOWNERS`](../../CODEOWNERS)
- Source gap: `elastic/oblt-aw/.github/workflows/obs-aw-event-schedule.yml@v0` is external and is not listed as a source.
