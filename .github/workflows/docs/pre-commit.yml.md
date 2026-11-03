# pre-commit

Runs the repository's pre-commit hooks on pull requests and on pushes to release branches, through the shared `elastic/oblt-actions/pre-commit` action.

## At a glance

| Item | Value |
|------|-------|
| Platform | GitHub Actions |
| Source | [`pre-commit.yml`](../pre-commit.yml) |
| Trigger | `pull_request` (all branches); `push` to `main`, `7.*`, `8.*`, `9.*` |
| Runner | `ubuntu-latest` |

## Purpose

Enforces the hooks declared in `.pre-commit-config.yaml` on every pull request and on pushes to the main and release branches.

## When it runs

- On every pull request.
- On push to `main` and to branches matching `7.*`, `8.*`, and `9.*`.

## Execution flow

| Job | Runs when | Depends on | What it does |
|-----|-----------|------------|--------------|
| `pre-commit` | Every matching event | None | Runs `elastic/oblt-actions/pre-commit@v1`, which runs the hooks from `.pre-commit-config.yaml`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Repository content and [`.pre-commit-config.yaml`](../../../.pre-commit-config.yaml). |
| Outputs | Pass or fail. |
| Permissions | Workflow token `contents: read`. |
| Secrets | None declared. |

## Operational notes

- The configuration pins `pre-commit-hooks` at `v6.0.0` and enables three hooks:
  - `check-executables-have-shebangs`.
  - `check-shebang-scripts-are-executable`, excluding `*.tmpl` and `*.tftpl` files.
  - `check-merge-conflict`, with `--assume-in-merge`.
- Source gap: the checkout and hook installation steps inside the external `elastic/oblt-actions/pre-commit` action are not verified here.

## Sources

- [`.github/workflows/pre-commit.yml`](../pre-commit.yml)
- [`.pre-commit-config.yaml`](../../../.pre-commit-config.yaml)
