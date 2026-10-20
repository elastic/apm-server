# bump-golang

Runs `updatecli` to bump the Go version on `main` and on the 7.17 branch, and opens pull requests with the bump.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`bump-golang.yml`](../bump-golang.yml) |
| Triggers | `workflow_dispatch`; schedule on a weekly basis |
| Token permissions | `contents: read` (workflow level). The `bump` job uses a GitHub App token with `contents: write` and `pull-requests: write`. |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN` |
| Runner | `ubuntu-latest` |

## Purpose

Keeps the Go toolchain version current. Only the latest active minor branch gets the new Go version label on `main`.

## When it runs

- On demand through `workflow_dispatch`.
- On a weekly schedule.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `labels` | Lists active branches excluding `7.17` and `main`, sorts them, and outputs the label `backport-<latest minor>`. |
| `bump` | Checks out `main`, creates a GitHub App token, and runs `updatecli` with `.ci/updatecli/bump-golang.yml`. The labels output from `labels` is passed as `GITHUB_LABELS`. |
| `bump-7` | Checks out `7.17` and runs `updatecli` with the same configuration. Labels are set to `backport-skip`. |
| `notify` | Runs when `always()`. Checks the status of `bump` and `bump-7` and posts to Slack when any of them failed. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | None. |
| Outputs | Pull requests that bump the Go version on `main` and `7.17`. |
| Permissions | GitHub App token for `bump`. |
| Secrets | Names only: `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN`. |

## Operational notes

- The `bump` job uses the latest active minor branch label, while `bump-7` always uses `backport-skip`.
- Source gap, not fixed here: the `bump-7` job passes `GITHUB_TOKEN: steps.get_token.outputs.token`, but no `get_token` step exists in that job. The token value is therefore expected to be empty. Confirm this before relying on the 7.17 bump.
- The Go version is taken from `.ci/updatecli/bump-golang.yml`. The external `elastic/oblt-actions` action is not verified here.

## Sources

- [`.github/workflows/bump-golang.yml`](../bump-golang.yml)
- [`.ci/updatecli/bump-golang.yml`](../../../.ci/updatecli/bump-golang.yml)
- [`.ci/updatecli/values.d/scm.yml`](../../../.ci/updatecli/values.d/scm.yml)
