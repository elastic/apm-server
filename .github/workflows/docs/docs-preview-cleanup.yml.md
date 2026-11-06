# docs-preview-cleanup

Cleans up documentation previews when a pull request targeting `main` is closed, by calling a reusable workflow from `elastic/docs-actions`.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/docs-preview-cleanup.yml`](../docs-preview-cleanup.yml) |
| Trigger | `pull_request_target`, activity type `closed`, base branch `main` |

## Purpose

The file removes documentation preview resources when their pull request is closed. The cleanup itself is performed by the external reusable workflow `elastic/docs-actions/.github/workflows/docs-preview-cleanup.yml@v1`; its steps are not in this repository. The file does not state what is removed.

## When it runs

- When a pull request targeting `main` is closed. The file does not check whether the pull request was merged, so both merged and unmerged closures run it.
- The file declares no path, label, or actor filters.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `cleanup` | On each closed pull request targeting `main` | None declared | Calls `elastic/docs-actions/.github/workflows/docs-preview-cleanup.yml@v1`. No inputs are passed. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Pull request event payload. No `with` inputs declared. |
| Outputs | Not declared in this file. |
| Permissions | Workflow level: `contents: none`, `deployments: write`, `id-token: write`. No job-level permissions declared. |
| Secrets | None passed explicitly. |

## Operational notes

- The workflow uses `pull_request_target`, which runs with base-branch context. The file does not check out pull request code.
- Source gap: the reusable workflow is external. What it removes, how it locates the preview, and any secrets it requires are not verifiable from this repository.

## Sources

- Workflow: [`.github/workflows/docs-preview-cleanup.yml`](../docs-preview-cleanup.yml)
- Source gap: `elastic/docs-actions` reusable workflows are external and are not listed as sources.
