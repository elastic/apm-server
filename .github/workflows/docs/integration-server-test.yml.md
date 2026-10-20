# integration-server-test

Runs APM Server integration tests against Elastic Cloud: upgrade tests for snapshot and backward-compatible upgrade paths, and standalone-to-managed tests. It runs on demand and on a weekday schedule.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`integration-server-test.yml`](../integration-server-test.yml) |
| Triggers | `workflow_dispatch` with three boolean inputs; `schedule` on weekdays |
| Concurrency | None |
| Token permissions | `contents: read` and `id-token: write` (workflow level) |
| Secrets | `EC_API_KEY` loaded from Google Secret Manager (name only), `SLACK_BOT_TOKEN` (notify job) |
| Runner | `ubuntu-latest` |

## Purpose

Checks that APM Server works across upgrades and in the standalone-to-managed path against real deployments.

## When it runs

- On demand through `workflow_dispatch`.
- On weekdays on a schedule.

## Inputs

| Input | Default | Effect |
|-------|---------|--------|
| `run-upgrade-tests` | `true` | Runs the snapshot upgrade job. |
| `run-upgrade-bc-tests` | `true` | Runs the backward-compatible upgrade job. |
| `run-standalone-tests` | `true` | Runs the standalone-to-managed job. |

A job is skipped when its input is `false`.

## Jobs and steps

| Job | Depends on | Matrix | Steps |
|-----|------------|--------|-------|
| `prepare` | None | None | Checks out the repository and runs the local composite [`generate-upgrade-paths`](../generate-upgrade-paths/action.yml). It outputs the snapshot and backward-compatible path lists. |
| `run-upgrade` | `prepare` | Each snapshot upgrade path times `Default` and `Reroute` | Runs the local composite [`setup-integration-server-test`](../setup-integration-server-test/action.yml), then `make integration-server-test/upgrade` with `SCENARIO` and `UPGRADE_PATH`. |
| `run-upgrade-bc` | `prepare` | Each backward-compatible upgrade path times `Default` and `Reroute` | Same as `run-upgrade`. Skipped when the path list is `[]`. |
| `run-standalone` | `prepare` | `Managed7`, `Managed8`, `Managed9` | Runs the setup composite, then `make integration-server-test/standalone` with `SCENARIO`. |
| `notify` | Four jobs above | None | Runs on `main` only. Checks dependent job results and posts a Slack message. |

The path generator runs `go run ./scripts/genpath` in `integrationservertest`, once for snapshot paths and once for backward-compatible paths. It targets `pro`.

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Outputs | Test results from the Makefile targets. Slack message on `main`. |
| Permissions | Read-only for the workflow token. `id-token: write` is used for Google Cloud authentication. |
| Secrets | Names only: `EC_API_KEY` (from Secret Manager), `SLACK_BOT_TOKEN`. |

## Operational notes

- Each matrix entry uses its own environment and `TF_VAR_CREATED_DATE`. Failures in one entry do not stop the others (`fail-fast: false`).
- The setup composite pins Terraform through the `TERRAFORM_VERSION` environment variable (`1.10.2`).
- The Makefile targets `integration-server-test/upgrade` and `integration-server-test/standalone`, and the Go code under `integrationservertest/`, were not inspected. These are source gaps.

## Sources

- [`.github/workflows/integration-server-test.yml`](../integration-server-test.yml)
- [`.github/workflows/generate-upgrade-paths/action.yml`](../generate-upgrade-paths/action.yml)
- [`.github/workflows/generate-upgrade-paths/script.sh`](../generate-upgrade-paths/script.sh)
- [`.github/workflows/setup-integration-server-test/action.yml`](../setup-integration-server-test/action.yml)
- Source gaps: the `integrationservertest/scripts/genpath` program and the Makefile integration targets were not inspected.
