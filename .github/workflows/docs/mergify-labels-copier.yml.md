# mergify-labels-copier

Copies labels to Mergify backport pull requests, using the external `elastic/oblt-actions/mergify/labels-copier@v1` action.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/mergify-labels-copier.yml`](../mergify-labels-copier.yml) |
| Trigger | `pull_request`, activity type `opened`; the job runs only when the head branch starts with `mergify/bp/` |

## Purpose

Labels Mergify backport pull requests so they carry the labels of their source change. The file excludes labels matching `^backport-*` from copying. The copying is done by the external action; its behavior is not in this repository. The file does not state further rationale.

## When it runs

- When a pull request is opened. The job's `if` condition runs it only when `github.head_ref` starts with `mergify/bp/`.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `mergify-backport-labels-copier` | Pull request opened from a `mergify/bp/` branch | None declared | Runs on `ubuntu-latest`. Runs `elastic/oblt-actions/mergify/labels-copier@v1` with `excluded-labels-regex: "^backport-*"`. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Pull request event payload. `excluded-labels-regex: "^backport-*"` passed to the action. |
| Outputs | Labels added to the backport pull request, per the external action's behavior. |
| Permissions | Workflow level: `contents: read`. Job level: `pull-requests: write`, `repository-projects: read`. |
| Secrets | None passed explicitly. |

## Operational notes

- Source gap: the behavior of `elastic/oblt-actions/mergify/labels-copier@v1` is external and not verifiable here, including which labels it copies beyond the exclusion regex.

## Sources

- Workflow: [`.github/workflows/mergify-labels-copier.yml`](../mergify-labels-copier.yml)
- Source gap: `elastic/oblt-actions/mergify/labels-copier@v1` is external and is not listed as a source.
