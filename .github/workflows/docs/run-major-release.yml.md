# run-major-release

Manually starts a major release. It mirrors [run-minor-release](./run-minor-release.yml.md) but uses `type: major` and `make major-release`.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`run-major-release.yml`](../run-major-release.yml) |
| Triggers | `workflow_dispatch` with one required input, `version` (semver `major.0.0`) |
| Concurrency | Group is the workflow name. |
| Token permissions | `contents: read` (workflow level). The `run-major` job uses a GitHub App token with `contents: write` and `pull-requests: write`. |
| Secrets | `SLACK_BOT_TOKEN`, `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE` |
| Runner | `ubuntu-latest` |

## Purpose

Automates the major release steps for the next major version.

## When it runs

Only on demand. There is no schedule.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `prepare` | Checks out the repository and runs the local composite [`prepare-release`](../prepare-release/action.yml) with `type: major`. |
| `run-major` | Posts the feature-freeze message to Slack, creates a GitHub App token, checks out with full history, configures git with `elastic/oblt-actions/git/setup@v1`, imports the GPG key, runs `make major-release`, then posts a success or failure message. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | `version`: the new version, normally the version on `main` (semver `major.0.0`). |
| Outputs | Release branch and tags from `make major-release`. Slack messages. |
| Permissions | GitHub App token for contents and pull requests. Commits are GPG signed. |
| Secrets | Names only: `SLACK_BOT_TOKEN`, `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`. |

## Operational notes

- The release steps are defined in `make major-release`, which is not inspected in this pass. This is a source gap.
- The success message names the release branch. The failure message links to the run logs.

## Sources

- [`.github/workflows/run-major-release.yml`](../run-major-release.yml)
- [`.github/workflows/prepare-release/action.yml`](../prepare-release/action.yml)
- Source gap: `make major-release` target in the [Makefile](../../../Makefile) not inspected.
