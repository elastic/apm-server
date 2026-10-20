# check-docker-compose

Runs `make check-docker-compose` on every active branch to verify the Docker Compose configuration.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`check-docker-compose.yml`](../check-docker-compose.yml) |
| Triggers | `workflow_dispatch`; schedule on a weekly basis |
| Token permissions | `contents: read` (workflow level) |
| Secrets | `SLACK_BOT_TOKEN` (only in the notify job on failure) |
| Runner | `ubuntu-latest` |

## Purpose

Catches Docker Compose drift or breakage on active release branches before someone needs them.

## When it runs

- On demand through `workflow_dispatch`.
- On a weekly schedule.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `filter` | Computes the active branch matrix with `elastic/oblt-actions/elastic/active-branches@v1`. Timeout is one minute. |
| `check-docker-compose` | For each branch, checks out the branch, sets up Go from `go.mod`, and runs `make check-docker-compose`. `fail-fast: false`. |
| `all-check-docker-compose` | Runs when `always()`. Checks the dependent job results and posts a Slack notification on failure. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | None. |
| Outputs | Pass or fail per branch. |
| Permissions | Read-only. |
| Secrets | Names only: `SLACK_BOT_TOKEN`. |

## Operational notes

- The Makefile target `check-docker-compose` is not inspected in this pass. This is a source gap.
- The aggregate job is the single signal to watch for failures across branches.

## Sources

- [`.github/workflows/check-docker-compose.yml`](../check-docker-compose.yml)
- Source gap: `make check-docker-compose` in the [Makefile](../../../Makefile) not inspected.
