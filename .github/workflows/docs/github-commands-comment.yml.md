# github-commands-comment

Posts a comment on newly opened pull requests that lists the available GitHub commands.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/github-commands-comment.yml`](../github-commands-comment.yml) |
| Trigger | `pull_request_target`, activity type `opened` |

## Purpose

Shows contributors which GitHub commands the repository supports when a pull request is opened. The comment is produced by the external action `elastic/oblt-actions/elastic/github-commands@v1`; its content is not in this repository.

## When it runs

- Each time a pull request is opened. The file declares no branch, path, label, or actor filters and no job condition.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `comment` | Pull request opened | None declared | Runs on `ubuntu-latest`. Runs `elastic/oblt-actions/elastic/github-commands@v1`. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Pull request event payload. No `with` inputs declared. |
| Outputs | One comment on the pull request, per the external action's behavior. |
| Permissions | Workflow level: `contents: read`. Job level: `pull-requests: write`. |
| Secrets | None passed explicitly. |

## Operational notes

- The workflow uses `pull_request_target`, so it runs with base-branch context. The file does not check out pull request code.
- Source gap: the behavior of `elastic/oblt-actions/elastic/github-commands@v1` is external and not verifiable here.

## Sources

- Workflow: [`.github/workflows/github-commands-comment.yml`](../github-commands-comment.yml)
- Source gap: `elastic/oblt-actions/elastic/github-commands@v1` is external and is not listed as a source.
