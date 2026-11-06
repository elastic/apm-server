# terraform-fmt

Checks that all Terraform files in the repository are formatted with `terraform fmt`, and fails when any file needs changes.

## At a glance

| Item | Value |
|------|-------|
| Platform | GitHub Actions |
| Source | [`terraform-fmt.yml`](../terraform-fmt.yml) (workflow name `Terraform Format`) |
| Trigger | `push` and `pull_request` when a `**.tf` file changes; `merge_group` |
| Runner | `ubuntu-latest` |

## Purpose

Enforces `terraform fmt` formatting on Terraform files before they merge.

## When it runs

- On a push or pull request that changes a `.tf` file.
- On every merge group event. The path filter is not set for `merge_group`.

## Execution flow

| Job | Runs when | Depends on | What it does |
|-----|-----------|------------|--------------|
| `terraform-fmt` | Every matching event | None | Checks out the repository, installs Terraform with `hashicorp/setup-terraform` (pinned to a commit SHA, no version set), and runs `terraform fmt -recursive -check -diff` on the whole repository. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Terraform files in the repository. |
| Outputs | Pass or fail. The diff of unformatted files is printed in the log. |
| Permissions | Workflow token `contents: read`. |
| Secrets | None. |

## Operational notes

- The `-check` flag makes the command fail without rewriting files.
- The path filter `**.tf` matches only files ending in `.tf`. Other Terraform-related files, such as `.tftpl`, do not trigger a push or pull request run.
- Source gap: the Terraform version installed by default is set by the external `hashicorp/setup-terraform` action and is not verified here.

## Sources

- [`.github/workflows/terraform-fmt.yml`](../terraform-fmt.yml)
