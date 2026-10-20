# pre-commit

Runs the repository's pre-commit hooks through the shared `elastic/oblt-actions` action.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`pre-commit.yml`](../pre-commit.yml) |
| Triggers | `pull_request` (all branches); `push` to `main`, `7.*`, `8.*`, `9.*` |
| Token permissions | `contents: read` (workflow level) |
| Secrets | None declared |
| Runner | `ubuntu-latest` |

## Purpose

Enforces the pre-commit hooks from the repository configuration on every pull request and on release branches.

## When it runs

- On every pull request.
- On push to `main` and to the `7.*`, `8.*`, and `9.*` branches.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `pre-commit` | Runs `elastic/oblt-actions/pre-commit@v1`. The hooks are defined by the repository's pre-commit configuration. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Repository content. |
| Outputs | Pass or fail of the hook run. |
| Permissions | Read-only. |
| Secrets | None. |

## Operational notes

- The hook list lives in `.pre-commit-config.yaml`, which is not inspected in this pass. This is a source gap.
- The action is external. Its checkout and install steps are not verified here.

## Sources

- [`.github/workflows/pre-commit.yml`](../pre-commit.yml)
- Source gap: `.pre-commit-config.yaml` and the external `elastic/oblt-actions/pre-commit` action are not verified here.
