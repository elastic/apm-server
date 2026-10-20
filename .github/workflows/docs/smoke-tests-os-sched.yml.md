# smoke-tests-os-sched

Runs the [smoke-tests-os](./smoke-tests-os.yml.md) workflow for every active branch except 7.17, on weekdays, and posts a Slack result on `main`.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`smoke-tests-os-sched.yml`](../smoke-tests-os-sched.yml) |
| Triggers | `workflow_dispatch` with a reminder checkbox; `schedule` on weekdays |
| Concurrency | None |
| Token permissions | `contents: read` (workflow level). The `smoke-tests-os` job adds `id-token: write`. |
| Secrets | Inherited (`secrets: inherit`). `SLACK_BOT_TOKEN` is used by the notify job. |
| Runner | `ubuntu-latest` |

## Purpose

Keeps the OS smoke tests running on a schedule across all supported release branches.

## When it runs

- On weekdays, on a schedule.
- On demand through `workflow_dispatch`.

## Inputs

| Input | Effect |
|-------|--------|
| `BRANCH_NOTE` | A reminder checkbox only. The workflow does not read it. |

To test a change on a branch, use [smoke-tests-os](./smoke-tests-os.yml.md) directly. The `BRANCH_NOTE` text says so.

## Jobs and steps

| Job | Depends on | Steps |
|-----|------------|-------|
| `prepare-smoke-tests-os` | None | Checks out the repository. Runs `elastic/oblt-actions/elastic/active-branches@v1` with `exclude-branches: '7.17'` to build a matrix of branches. |
| `smoke-tests-os` | `prepare-smoke-tests-os` | Calls [`smoke-tests-os.yml`](../smoke-tests-os.yml) once per branch in the matrix. `fail-fast: false`. |
| `notify` | `smoke-tests-os` | Runs on `main` only, including when the matrix jobs fail. Checks the dependent job results and posts a Slack message to `#apm-server`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Outputs | Per-branch test results from the called workflow. Slack message on `main`. |
| Permissions | Read-only for the workflow token. `id-token: write` for the called jobs. |
| Secrets | Names only, inherited by the called workflow: `EC_API_KEY`, and `SLACK_BOT_TOKEN` for the notify job. |

## Operational notes

- The active branch list comes from the external `active-branches` action. Its rules are not verified here. This is a source gap.
- Scheduled runs use the default branch's copy of this workflow. Changes on a feature branch take effect only after they are merged, which is why the reminder exists.

## Sources

- [`.github/workflows/smoke-tests-os-sched.yml`](../smoke-tests-os-sched.yml)
- [`.github/workflows/smoke-tests-os.yml`](../smoke-tests-os.yml)
- Source gap: the external `elastic/oblt-actions/elastic/active-branches` action is not verified here.
