# smoke-tests-os

Runs the OS smoke tests from `testing/smoke/supported-os` for one branch. It is a reusable workflow, and it can also be started by hand.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`smoke-tests-os.yml`](../smoke-tests-os.yml) |
| Triggers | `workflow_call` with a required `branch` input; `workflow_dispatch` |
| Concurrency | None |
| Token permissions | `contents: read` (workflow level). The `smoke-tests-os` job adds `id-token: write`. |
| Secrets | `EC_API_KEY` loaded from Google Secret Manager (name only) |
| Runner | `ubuntu-latest` |

## Purpose

Checks that the supported-OS smoke suite passes for a given branch against a SNAPSHOT build.

## When it runs

- When [smoke-tests-os-sched](./smoke-tests-os-sched.yml.md) calls it for each active branch.
- On demand through `workflow_dispatch`.

## Inputs

| Input | Required | Effect |
|-------|----------|--------|
| `branch` | Yes for `workflow_call` | Branch to check out. |

## Jobs and steps

| Job | Steps |
|-----|-------|
| `prepare` | Generates a timestamp for the environment name and outputs it. |
| `smoke-tests-os` | Checks out `inputs.branch`. Installs Terraform. Reads the version with `make get-version`. Runs the local composite [`setup-cluster-env`](../setup-cluster-env/action.yml), which sets up Go. Authenticates to AWS and Google Cloud. Loads `EC_API_KEY`. Runs `./test.sh <version>-SNAPSHOT` in `testing/smoke/supported-os`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Outputs | Test results in the job log. |
| Permissions | Read-only for the workflow token. `id-token: write` for cloud authentication. |
| Secrets | Name only: `EC_API_KEY`. |

## Operational notes

- The `branch` input is required only for `workflow_call`. For a manual run, the input is empty, so the checkout falls back to the workflow's own ref. The `smoke-tests-os-sched` page explains the related dispatch behavior.
- `TF_VAR_BRANCH` is set from `github.ref_name`, not from the `branch` input. This is an inconsistency in source that affects only the environment label.
- `test.sh` and the supported-OS test code were not inspected. This is a source gap.

## Sources

- [`.github/workflows/smoke-tests-os.yml`](../smoke-tests-os.yml)
- [`.github/workflows/setup-cluster-env/action.yml`](../setup-cluster-env/action.yml)
- Source gap: `testing/smoke/supported-os/test.sh` was not inspected.
