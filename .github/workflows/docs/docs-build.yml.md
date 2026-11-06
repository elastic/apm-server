# docs-build

Builds the repository's documentation with Vale linting enabled, by calling a reusable workflow from `elastic/docs-actions`.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/docs-build.yml`](../docs-build.yml) |
| Trigger | `pull_request` to `main` (types `opened`, `synchronize`, `reopened`); `push` to `main`; `merge_group` |

## Purpose

The file builds the documentation for pull requests, pushes to `main`, and merge queue checks, with Vale prose linting enabled. The build itself runs in the external reusable workflow `elastic/docs-actions/.github/workflows/docs-build.yml@v1`; its steps are not in this repository. The repository's documentation source includes [`docs/docset.yml`](../../../docs/docset.yml). How the reusable workflow consumes it is not verified here. The file does not state a further reason for the build.

## When it runs

- On `pull_request` events to `main` with types `opened`, `synchronize`, or `reopened`.
- On `push` to `main`.
- On `merge_group` events.
- The file declares no path filter, so any change that matches these events starts the job. Whether the reusable workflow limits the build to documentation changes is a source gap.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `build` | On each event listed under When it runs | None declared | Calls `elastic/docs-actions/.github/workflows/docs-build.yml@v1` with `enable-vale-linting: true`. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | `enable-vale-linting: true` passed to the reusable workflow. |
| Outputs | Not declared in this file. The reusable workflow's outputs are not verifiable here. |
| Permissions | Workflow level: `contents: read`, `pull-requests: read`. No job-level permissions declared. |
| Secrets | None passed explicitly. |

## Operational notes

- The reusable workflow is pinned to the `v1` ref of the external `elastic/docs-actions` repository. Its behavior, outputs, and any secrets it requires are not verifiable here (source gap).
- The workflow uses `pull_request`, not `pull_request_target`.

## Sources

- Workflow: [`.github/workflows/docs-build.yml`](../docs-build.yml)
- Documentation source referenced by the build: [`docs/docset.yml`](../../../docs/docset.yml)
- Source gap: `elastic/docs-actions/.github/workflows/docs-build.yml@v1` is external and is not listed as a source.

