# catalog-info

Validates the repository's `catalog-info.yaml` on pull requests to `main` that change it.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/catalog-info.yml`](../catalog-info.yml) |
| Trigger | `pull_request` to `main` with a path filter on `catalog-info.yaml` |

## Purpose

Validates the service catalog entry in [`catalog-info.yaml`](../../../catalog-info.yaml) before it changes on `main`. The validation is performed by the external action `elastic/oblt-actions/elastic/validate-catalog@v1`; its rules are not in this repository.

## When it runs

- On `pull_request` events targeting `main` when the pull request changes `catalog-info.yaml`. The path filter is the only filter declared.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `validate` | Pull request to `main` changing `catalog-info.yaml` | None declared | Runs on `ubuntu-latest`. Checks out the repository with `actions/checkout@v7`, then runs `elastic/oblt-actions/elastic/validate-catalog@v1`. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | The changed `catalog-info.yaml` in the checked-out repository. |
| Outputs | Validation result only. The file declares no artifacts. |
| Permissions | Job level: `contents: read`, `packages: read`. |
| Secrets | None passed explicitly. |

## Operational notes

- Source gap: the validation rules and any extra access used by `elastic/oblt-actions/elastic/validate-catalog@v1` are external and not verifiable here.

## Sources

- Workflow: [`.github/workflows/catalog-info.yml`](../catalog-info.yml)
- Validated file: [`catalog-info.yaml`](../../../catalog-info.yaml)
