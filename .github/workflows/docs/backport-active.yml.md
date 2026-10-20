# backport-active

Backports a merged pull request to every active branch that has a matching `backport-active-*` label.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`backport-active.yml`](../backport-active.yml) |
| Triggers | `pull_request_target`: `closed`, `labeled`; base branch `main` |
| Token permissions | `pull-requests: write`, `contents: read` |
| Secrets | None declared in the workflow. The `elastic/oblt-actions` action may use its own inputs. |
| Runner | `ubuntu-latest` |

## Purpose

Automates backports for active release branches after a pull request is merged into `main`.

## When it runs

- When a pull request targeting `main` is closed **and merged** and already carries a `backport-active-*` label.
- When a `backport-active-*` label is added to a merged pull request.

Closed but unmerged pull requests and other labels are skipped by the job `if` condition.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `backport` | Runs the composite action `elastic/oblt-actions/github/backport-active@v1`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Pull request event payload. |
| Outputs | Backport pull requests created by the action. |
| Permissions | Pull request write, so the job can open backport PRs. |
| Secrets | None declared. |

## Operational notes

- This workflow uses `pull_request_target`, so it runs with base-branch permissions. It does not check out pull request code.
- Source gap: the behavior of `elastic/oblt-actions/github/backport-active@v1` is external and not documented here.

## Sources

- [`.github/workflows/backport-active.yml`](../backport-active.yml)
