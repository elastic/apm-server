# trigger-obs-aw-issues

Forwards issue events, and manual dispatches, to the Observability Agentic Workflow handler in the external `elastic/oblt-aw` repository.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/trigger-obs-aw-issues.yml`](../trigger-obs-aw-issues.yml) |
| Trigger | `issues`, activity types `opened`, `labeled`, `edited`; `workflow_dispatch` |

## Purpose

This file is the repository-side client for the Observability Agentic Workflow for issues. It declares the events and permissions, then calls `elastic/oblt-aw/.github/workflows/obs-aw-event-issues.yml@v0`, where the handling happens. The file is owned by `@elastic/observablt-ci` according to the CODEOWNERS entry for `trigger-obs-aw-*` files.

## When it runs

- When an issue is opened, labeled, or edited.
- When the workflow is started manually with `workflow_dispatch`.
- The file declares no label, actor, or body filters, and no job condition.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `run-obs-aw-issues` | On each matching issue event or manual dispatch | None declared | Calls `elastic/oblt-aw/.github/workflows/obs-aw-event-issues.yml@v0` with the job permissions below. No inputs or secrets are passed. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Issue event payload or manual dispatch. No `with` inputs declared. |
| Outputs | Not declared in this file. |
| Permissions | Workflow level: `contents: read`. Job level: `actions: read`, `contents: write`, `copilot-requests: write`, `discussions: write`, `id-token: write`, `issues: write`, `pull-requests: write`. |
| Secrets | None passed explicitly. |

## Operational notes

- The called workflow is pinned to the `v0` ref of the external `elastic/oblt-aw` repository. Its behavior and any secrets it requires are not verifiable from this repository (source gap).
- The `uses:` line carries a `ratchet:exclude` marker. The tool that reads this marker is not checked in here.

## Sources

- Workflow: [`.github/workflows/trigger-obs-aw-issues.yml`](../trigger-obs-aw-issues.yml)
- Ownership: [`.github/CODEOWNERS`](../../CODEOWNERS)
- Source gap: `elastic/oblt-aw/.github/workflows/obs-aw-event-issues.yml@v0` is external and is not listed as a source.
