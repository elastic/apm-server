# catalog-info

Validates the `catalog-info.yaml` file on pull requests that change it.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`catalog-info.yml`](../catalog-info.yml) |
| Triggers | `pull_request` to `main` when `catalog-info.yaml` changes |
| Token permissions | `contents: read`, `packages: read` (job level) |
| Secrets | None |
| Runner | `ubuntu-latest` |

## Purpose

Prevents an invalid service catalog entry from merging into `main`.

## When it runs

Only for pull requests targeting `main` that change `catalog-info.yaml`.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `validate` | Checks out the repository, then runs `elastic/oblt-actions/elastic/validate-catalog@v1`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | The changed `catalog-info.yaml`. |
| Outputs | Validation result only. |
| Permissions | Read-only. |
| Secrets | None. |

## Sources

- [`.github/workflows/catalog-info.yml`](../catalog-info.yml)
- `catalog-info.yaml` at the repository root is the validated file.
