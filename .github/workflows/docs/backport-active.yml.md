# backport-active

Backports a merged pull request to the active release branches that its `backport-active-*` label selects.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/backport-active.yml`](../backport-active.yml) |
| Trigger | `pull_request_target`, activity types `closed` and `labeled`, base branch `main` |

## Purpose

Automates backports to active branches after a pull request is merged into `main`. The backport work is done by the external composite action `elastic/oblt-actions/github/backport-active@v1`; its behavior is not in this repository. The file does not state further rationale.

## When it runs

- When a pull request targeting `main` is closed and merged, and already carries a label starting with `backport-active-`.
- When a label starting with `backport-active-` is added to a pull request that is merged.
- The job `if` condition skips closed pull requests that were not merged and any other label.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `backport` | Merged pull request with a `backport-active-*` label, on close or label addition | None declared | Runs on `ubuntu-latest`. Uses `elastic/oblt-actions/github/backport-active@v1` with no inputs passed from this file. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Pull request event payload. No `with` inputs declared. |
| Outputs | Backport pull requests created by the action, per its external behavior (not verifiable here). |
| Permissions | Workflow level: `pull-requests: write`, `contents: read`. No job-level permissions declared. |
| Secrets | None passed explicitly. |

## Operational notes

- The workflow uses `pull_request_target`, so it runs with base-branch context. The file does not check out pull request code.
- Source gap: the behavior of `elastic/oblt-actions/github/backport-active@v1` is external. Which branches it targets and any secrets it requires are not verifiable here.

## Sources

- Workflow: [`.github/workflows/backport-active.yml`](../backport-active.yml)
- Source gap: `elastic/oblt-actions/github/backport-active@v1` is external and is not listed as a source.
