# run-patch-release

Manually starts a patch release. It checks out the release branch and runs `make patch-release`.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`run-patch-release.yml`](../run-patch-release.yml) |
| Triggers | `workflow_dispatch` with one required input, `version` (the current version to release, semver `major.minor.patch`) |
| Concurrency | Group is the workflow name. |
| Token permissions | `contents: read` (workflow level). The `run-patch` job uses a GitHub App token with `contents: write` and `pull-requests: write`. |
| Secrets | `SLACK_BOT_TOKEN`, `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE` |
| Runner | `ubuntu-latest` |

## Purpose

Automates a patch release from an existing release branch.

## When it runs

Only on demand. There is no schedule.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `prepare` | Checks out the repository and runs the local composite [`prepare-release`](../prepare-release/action.yml) with `type: patch`. |
| `run-patch` | Creates a GitHub App token, checks out the release branch with full history using `ref: RELEASE_BRANCH`, configures git, imports the GPG key, runs `make patch-release`, then posts a success or failure message. |

On success, the workflow posts that the feature freeze is today and that the relevant PRs and issues have been created. On failure, it posts a message with a link to the logs.

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | `version`: the current version to release. |
| Outputs | Patch release from `make patch-release`. Slack messages. |
| Permissions | GitHub App token for contents and pull requests. Commits are GPG signed. |
| Secrets | Names only: `SLACK_BOT_TOKEN`, `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`. |

## Operational notes

- The checkout uses the release branch so the release is made with that branch's Makefile. The source comment says so.
- Unlike the minor and major workflows, which post the feature-freeze message before the make target runs, this workflow posts it only after the release succeeds.
- Source gap: `make patch-release` target in the [Makefile](../../../Makefile) not inspected.

## Sources

- [`.github/workflows/run-patch-release.yml`](../run-patch-release.yml)
- [`.github/workflows/prepare-release/action.yml`](../prepare-release/action.yml)
- Source gap: `make patch-release` target in the [Makefile](../../../Makefile) not inspected.
