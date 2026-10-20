# mergify-labels-copier

Copies labels from a merged pull request's original change to the Mergify backport pull request it creates.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`mergify-labels-copier.yml`](../mergify-labels-copier.yml) |
| Triggers | `pull_request`: `opened` |
| Token permissions | `contents: read` (workflow level); `pull-requests: write`, `repository-projects: read` (job level) |
| Secrets | None declared |
| Runner | `ubuntu-latest` |

## Purpose

Keeps backport pull requests created by Mergify labeled like their source change, except for `backport-*` labels.

## When it runs

Only when a newly opened pull request has a head branch that starts with `mergify/bp/`.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `mergify-backport-labels-copier` | Runs `elastic/oblt-actions/mergify/labels-copier@v1` with `excluded-labels-regex: "^backport-*"`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Pull request event payload. |
| Outputs | Labels added to the backport pull request. |
| Permissions | Pull request write. |
| Secrets | None declared. |

## Operational notes

- Labels matching `^backport-*` are not copied, so backport labels are not propagated to backport pull requests.

## Sources

- [`.github/workflows/mergify-labels-copier.yml`](../mergify-labels-copier.yml)
