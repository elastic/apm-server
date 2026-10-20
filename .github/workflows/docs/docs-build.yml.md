# docs-build

Builds a documentation preview for changes under `docs/` by calling a reusable workflow from `elastic/docs-builder`.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`docs-build.yml`](../docs-build.yml) |
| Triggers | `push` to `main`; `pull_request_target`; `merge_group` |
| Token permissions | Set by the reusable workflow. The caller requests `deployments: write`, `id-token: write`, `contents: read`, `pull-requests: write`. |
| Secrets | None declared in this file. |
| Runner | Set by the reusable workflow. |

## Purpose

Lets reviewers preview rendered documentation before merge.

## When it runs

- On push to `main`.
- On `pull_request_target` events for pull requests.
- On merge group events.

The caller passes `path-pattern: docs/**`, so the reusable workflow is responsible for limiting the build to documentation changes.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `docs-preview` | Calls `elastic/docs-builder/.github/workflows/preview-build.yml@main`. All steps live in the external workflow. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | `path-pattern: docs/**`. |
| Outputs | A preview deployment for documentation changes. |
| Permissions | Requested at the caller level and passed to the reusable workflow. |
| Secrets | None declared in this file. |

## Operational notes

- The reusable workflow is pinned to `@main`. Its behavior is not verified in this repository.
- `pull_request_target` runs with base-branch permissions. This page does not check out pull request code.

## Sources

- [`.github/workflows/docs-build.yml`](../docs-build.yml)
- Source gap: the `elastic/docs-builder` preview workflow is external and not verified here.

