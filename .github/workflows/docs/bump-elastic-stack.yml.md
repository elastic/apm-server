# bump-elastic-stack

Runs `updatecli` to bump the Elastic Stack snapshot build used by the test infrastructure on each active release branch.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`bump-elastic-stack.yml`](../bump-elastic-stack.yml) |
| Triggers | `workflow_dispatch`; schedule on weekdays |
| Token permissions | `contents: read` (workflow level). The job token is a GitHub App token with `contents: write` and `pull-requests: write`. |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN` |
| Runner | `ubuntu-latest` |

## Purpose

Keeps the Elastic Stack snapshot references up to date on every active branch, opening a pull request with the new build id.

## When it runs

- On demand through `workflow_dispatch`.
- On a weekday schedule.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `filter` | Computes the active branch matrix with `elastic/oblt-actions/elastic/active-branches@v1`. Timeout is one minute. |
| `bump-elastic-stack` | For each branch in the matrix, checks out the branch, creates a GitHub App token, and runs `updatecli --experimental apply` with `.ci/updatecli/bump-elastic-stack-snapshot.yml` and `.ci/updatecli/values.d/scm.yml`. On failure, posts to Slack. |

The matrix uses `fail-fast: false`, so one branch failing does not stop the others.

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | None. The branch list comes from the active-branches action. |
| Outputs | Pull requests on each active branch that updates the snapshot references. |
| Permissions | GitHub App token with contents and pull request write. |
| Secrets | Names only: `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `SLACK_BOT_TOKEN`. |

## Operational notes

- The updatecli configuration fetches the snapshot build id from the Elastic artifacts snapshot index for each branch. It then updates `docker-compose.yml` and the Kubernetes stack manifests under `testing/infra/k8s/base/stack/`. See [`bump-elastic-stack-snapshot.yml`](../../../.ci/updatecli/bump-elastic-stack-snapshot.yml).
- Pull requests created by the updatecli run carry the labels defined in the updatecli values.
- Source gap: the snapshot index and the external `elastic/oblt-actions` action are not part of this repository and are not verified here.

## Sources

- [`.github/workflows/bump-elastic-stack.yml`](../bump-elastic-stack.yml)
- [`.ci/updatecli/bump-elastic-stack-snapshot.yml`](../../../.ci/updatecli/bump-elastic-stack-snapshot.yml)
- [`.ci/updatecli/values.d/scm.yml`](../../../.ci/updatecli/values.d/scm.yml)
