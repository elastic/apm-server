# update-beats

Runs `updatecli` to update the Beats dependency on each active branch.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`update-beats.yml`](../update-beats.yml) |
| Triggers | `workflow_dispatch`; schedule on weekdays |
| Token permissions | `contents: read` (workflow level). The job token is a GitHub App token with `contents: write` and `pull-requests: write`. |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN` |
| Runner | `ubuntu-latest` |

## Purpose

Keeps the Beats module version in sync on every active branch.

## When it runs

- On demand through `workflow_dispatch`.
- On a weekday schedule.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `filter` | Computes the active branch matrix with `elastic/oblt-actions/elastic/active-branches@v1`. |
| `bump` | For each branch, checks out the branch, creates a GitHub App token, sets up Go from `go.mod`, and runs `updatecli --experimental apply` with `.ci/updatecli/update-beats.yml`. On failure, posts to Slack. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | None. |
| Outputs | Pull requests on each active branch that update Beats. |
| Permissions | GitHub App token with contents and pull request write. |
| Secrets | Names only: `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN`. |

## Operational notes

- The matrix uses `fail-fast: false`.
- The updatecli configuration calls `.ci/scripts/update-beats.sh` with the branch name. See [`update-beats.yml` config](../../../.ci/updatecli/update-beats.yml).
- Go is set up only for the updatecli step, using `go.mod` as the version source.
- Source gap: the Beats source of truth used by the script is not verified here.

## Sources

- [`.github/workflows/update-beats.yml`](../update-beats.yml)
- [`.ci/updatecli/update-beats.yml`](../../../.ci/updatecli/update-beats.yml)
- [`.ci/scripts/update-beats.sh`](../../../.ci/scripts/update-beats.sh)
- [`.ci/updatecli/values.d/scm.yml`](../../../.ci/updatecli/values.d/scm.yml)
