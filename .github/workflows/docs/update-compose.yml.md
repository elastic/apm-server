# update-compose

Runs `updatecli` in compose mode to diff and apply updates to the Docker Compose service images on `main`.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`update-compose.yml`](../update-compose.yml) |
| Triggers | `workflow_dispatch`; schedule daily |
| Token permissions | `contents: read`, `packages: read` (job level). The updatecli token is a GitHub App token scoped to this repository with `contents: write` and `pull-requests: write`. |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN`, `GITHUB_TOKEN` (built-in, used for the GHCR login) |
| Runner | `ubuntu-latest` |

## Purpose

Keeps `docker-compose.yml` service images current without manual edits.

## When it runs

- On demand through `workflow_dispatch`.
- Once a day.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `compose` | Checks out the repository, creates a GitHub App token limited to `apm-server`, logs in to `ghcr.io`, runs `updatecli` `compose diff`, then `compose apply`. On failure, posts to Slack. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | None. The configuration comes from `updatecli-compose.yaml`. |
| Outputs | A pull request when `compose apply` finds image updates. |
| Permissions | Read access to packages for the GHCR login. GitHub App token for contents and pull requests. |
| Secrets | Names only: `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN`. |

## Operational notes

- The `diff` step runs before `apply` and is useful for reading the expected change in the log.
- The configuration lives in `updatecli-compose.yaml` at the repository root and in `.ci/updatecli/values.d/updatecli-compose.yml`.
- Source gap: the external `elastic/oblt-actions/updatecli` compose mode is not verified here.

## Sources

- [`.github/workflows/update-compose.yml`](../update-compose.yml)
- [`updatecli-compose.yaml`](../../../updatecli-compose.yaml)
- [`.ci/updatecli/values.d/updatecli-compose.yml`](../../../.ci/updatecli/values.d/updatecli-compose.yml)
