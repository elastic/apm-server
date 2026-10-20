# update-compose

Runs `updatecli` in compose mode daily to update image references in the configured policies and open a pull request when updates are found.

## At a glance

| Item | Value |
|------|-------|
| Platform | GitHub Actions |
| Source | [`update-compose.yml`](../update-compose.yml) |
| Trigger | `workflow_dispatch`; schedule daily |
| Runner | `ubuntu-latest` |

## Purpose

Keeps image references used by this repository current through `updatecli` compose policies, without manual edits.

## When it runs

- On demand through `workflow_dispatch`.
- On a daily schedule.

## Execution flow

| Job | Runs when | Depends on | What it does |
|-----|-----------|------------|--------------|
| `compose` | Every run | None | Checks out the repository, creates a GitHub App token scoped to the `apm-server` repository, logs in to `ghcr.io` with `GITHUB_TOKEN`, runs `updatecli compose diff`, then `updatecli compose apply`. On failure, posts to Slack. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | None. The policies and values come from `updatecli-compose.yaml`. |
| Outputs | A pull request when `compose apply` finds updates. |
| Permissions | Job token `contents: read` and `packages: read`, used for the GHCR login. The App token requests `contents: write` and `pull-requests: write`. |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN`, and `GITHUB_TOKEN` (built-in, used for the GHCR login). Names only. |

## Operational notes

- The `diff` step runs before `apply`, so the expected changes appear in the log.
- `updatecli-compose.yaml` declares two policies, both pinned by digest on GHCR:
  - `ironbank/templates`, with values from `scm.yml` and `ironbank.yml`. The ironbank values set the path `packaging/ironbank` and the labels `dependencies` and `backport-active-all`.
  - `autodiscovery/updatecli`, with values from `scm.yml` and `updatecli-compose.yml`. It updates the updatecli policies themselves.
- Source gap: the contents of the two GHCR policies are external and are not verified here.
- Source gap: the external `elastic/oblt-actions/updatecli/run` action and Slack action are not verified here.

## Sources

- [`.github/workflows/update-compose.yml`](../update-compose.yml)
- [`updatecli-compose.yaml`](../../../updatecli-compose.yaml)
- [`.ci/updatecli/values.d/scm.yml`](../../../.ci/updatecli/values.d/scm.yml)
- [`.ci/updatecli/values.d/ironbank.yml`](../../../.ci/updatecli/values.d/ironbank.yml)
- [`.ci/updatecli/values.d/updatecli-compose.yml`](../../../.ci/updatecli/values.d/updatecli-compose.yml)
