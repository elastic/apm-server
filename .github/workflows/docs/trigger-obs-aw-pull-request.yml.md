# trigger-obs-aw-pull-request

Forwards pull request events to the Observability Agentic Workflow handler in the external `elastic/oblt-aw` repository.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/trigger-obs-aw-pull-request.yml`](../trigger-obs-aw-pull-request.yml) |
| Trigger | `pull_request`, activity types `opened`, `synchronize`, `reopened`, `labeled` |

## Purpose

This file is the repository-side client for the Observability Agentic Workflow for pull requests. It declares the events and permissions, then calls `elastic/oblt-aw/.github/workflows/obs-aw-event-pull-request.yml@v0`, where the handling happens. The file is owned by `@elastic/observablt-ci` according to the CODEOWNERS entry for `trigger-obs-aw-*` files.

## When it runs

- When a pull request is opened, receives new commits (`synchronize`), is reopened, or is labeled.
- The file declares no branch, path, label, or actor filters, and no job condition.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `run-obs-aw-pull-request` | On each matching pull request event | None declared | Calls `elastic/oblt-aw/.github/workflows/obs-aw-event-pull-request.yml@v0` with the job permissions below. No inputs or secrets are passed. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Pull request event payload. No `with` inputs declared. |
| Outputs | Not declared in this file. |
| Permissions | Workflow level: `contents: read`. Job level: `actions: read`, `contents: write`, `copilot-requests: write`, `discussions: write`, `id-token: write`, `issues: write`, `pull-requests: write`. |
| Secrets | None passed explicitly. |

## Operational notes

- The workflow uses `pull_request`, not `pull_request_target`. Behavior for pull requests from forks, and any secrets the called workflow requires, are not verifiable from this repository (source gap).
- The called workflow is pinned to the `v0` ref of the external `elastic/oblt-aw` repository.
- The `uses:` line carries a `ratchet:exclude` marker. The tool that reads this marker is not checked in here.

## Sources

- Workflow: [`.github/workflows/trigger-obs-aw-pull-request.yml`](../trigger-obs-aw-pull-request.yml)
- Ownership: [`.github/CODEOWNERS`](../../CODEOWNERS)
- Source gap: `elastic/oblt-aw/.github/workflows/obs-aw-event-pull-request.yml@v0` is external and is not listed as a source.
