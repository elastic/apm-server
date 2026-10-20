# github-commands-comment

Posts a comment on newly opened pull requests that lists the available GitHub commands.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`github-commands-comment.yml`](../github-commands-comment.yml) |
| Triggers | `pull_request_target`: `opened` |
| Token permissions | `contents: read` (workflow level); `pull-requests: write` (job level) |
| Secrets | None declared |
| Runner | `ubuntu-latest` |

## Purpose

Shows contributors which slash commands the repository supports when a pull request is opened.

## When it runs

Once for each newly opened pull request, including pull requests from forks.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `comment` | Runs `elastic/oblt-actions/elastic/github-commands@v1`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Pull request event payload. |
| Outputs | One comment on the pull request. |
| Permissions | Pull request write. |
| Secrets | None declared. |

## Operational notes

- This workflow uses `pull_request_target` and does not check out pull request code, so forks cannot run code with the write token.

## Sources

- [`.github/workflows/github-commands-comment.yml`](../github-commands-comment.yml)
