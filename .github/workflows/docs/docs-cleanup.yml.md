# docs-cleanup

Removes the documentation preview created by `docs-build` when a pull request is closed.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`docs-cleanup.yml`](../docs-cleanup.yml) |
| Triggers | `pull_request_target` on `closed` |
| Token permissions | `contents: none`, `id-token: write`, `deployments: write` |
| Secrets | None declared in this file. |
| Runner | Set by the reusable workflow. |

## Purpose

Deletes the preview deployment so closed pull requests do not leave stale previews behind.

## When it runs

When any pull request targeting this repository is closed, whether merged or not.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `docs-preview` | Calls `elastic/docs-builder/.github/workflows/preview-cleanup.yml@main`. All steps live in the external workflow. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Pull request event payload. |
| Outputs | Deleted preview deployment. |
| Permissions | `contents: none` for the caller. Deployment and OIDC permissions are passed to the reusable workflow. |
| Secrets | None declared in this file. |

## Operational notes

- The reusable workflow is pinned to `@main`. Its behavior is not verified in this repository.
- Pairs with [docs-build](./docs-build.yml.md).

## Sources

- [`.github/workflows/docs-cleanup.yml`](../docs-cleanup.yml)
- Source gap: the `elastic/docs-builder` preview-cleanup workflow is external and not verified here.
