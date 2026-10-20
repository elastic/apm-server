# terraform-fmt

Checks that Terraform files are formatted.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`terraform-fmt.yml`](../terraform-fmt.yml) |
| Triggers | `push` and `pull_request` when `**.tf` files change; `merge_group` |
| Token permissions | `contents: read` (workflow level) |
| Secrets | None |
| Runner | `ubuntu-latest` |

## Purpose

Fails when Terraform files are not formatted with `terraform fmt`.

## When it runs

- On push or pull request that changes a `.tf` file.
- On merge group events. The path filter does not apply to merge groups, so this job runs for every merge group.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `terraform-fmt` | Checks out the repository, installs Terraform with `hashicorp/setup-terraform`, runs `terraform fmt -recursive -check -diff`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Terraform files in the repository. |
| Outputs | Pass or fail with the formatting diff in the log. |
| Permissions | Read-only. |
| Secrets | None. |

## Sources

- [`.github/workflows/terraform-fmt.yml`](../terraform-fmt.yml)
