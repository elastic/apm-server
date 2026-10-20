# run-minor-release

Manually starts a minor release: posts the feature-freeze message, then runs `make minor-release` to cut the release branch and update the docs and references.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`run-minor-release.yml`](../run-minor-release.yml) |
| Triggers | `workflow_dispatch` with one required input, `version` (semver `major.minor.0`) |
| Concurrency | Group is the workflow name. `cancel-in-progress` is not set. |
| Token permissions | `contents: read` (workflow level). The `run-minor` job uses a GitHub App token with `contents: write` and `pull-requests: write`. |
| Secrets | `SLACK_BOT_TOKEN`, `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE` |
| Runner | `ubuntu-latest` |

## Purpose

Automates the minor release steps that are not manual. The posted Slack messages record the freeze and the release.

## When it runs

Only on demand. There is no schedule.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `prepare` | Checks out the repository and runs the local composite [`prepare-release`](../prepare-release/action.yml) with `type: minor`. Outputs the release branch, type, version, and Slack thread timestamp. |
| `run-minor` | Posts the feature-freeze message to Slack. Creates a GitHub App token. Checks out with full history. Configures the git user with `elastic/oblt-actions/git/setup@v1`. Imports the GPG key. Runs `make minor-release`. Posts a success or failure message. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | `version`: the new version, normally the version on `main`. |
| Outputs | Release branch and tags created by `make minor-release`. Slack messages in the release thread. |
| Permissions | GitHub App token for contents and pull requests. Commits are GPG signed. |
| Secrets | Names only: `SLACK_BOT_TOKEN`, `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`, `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`. |

## Operational notes

- Because the `make` target does the release work, the exact steps are defined in the [Makefile](../../../Makefile), which is not inspected in this pass. This is a source gap.
- The release is created from the `main` branch version, as the input description says.

## Sources

- [`.github/workflows/run-minor-release.yml`](../run-minor-release.yml)
- [`.github/workflows/prepare-release/action.yml`](../prepare-release/action.yml)
- Source gap: `make minor-release` target in the [Makefile](../../../Makefile) not inspected.
