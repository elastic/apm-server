# ci-docs

Mirror of the [ci](./ci.yml.md) workflow for documentation-only changes. It runs for pull requests that change only Markdown, AsciiDoc, PNG, or SVG files, so required checks are reported without running the full build.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`ci-docs.yml`](../ci-docs.yml) |
| Workflow name | `ci` (same name as `ci.yml`) |
| Triggers | `pull_request` with a paths-ignore filter that matches docs-only changes; `merge_group` |
| Concurrency | None |
| Token permissions | `contents: read` (workflow level) |
| Secrets | None |
| Runner | `macos-latest` and `windows-latest` for the `test` job; `ubuntu-latest` for `system-test` and `lint` |

## Purpose

Lets pull requests that change only documentation pass the same required check names that [ci](./ci.yml.md) reports, without running the code build.

## When it runs

- On pull requests where every changed file is Markdown, AsciiDoc, PNG, or SVG.
- On merge group events.

The paths filter is a negated list: it ignores all files except `**.md`, `**.asciidoc`, `**.png`, and `**.svg`. The source comment says this avoids a run when code and docs change together. It must match the ignore list in `ci.yml`.

## Jobs and steps

| Job | Runs on | Steps |
|-----|---------|-------|
| `test` | `macos-latest`, `windows-latest` (matrix) | Prints that no tests are required. |
| `system-test` | `ubuntu-latest` | Prints that no tests are required. |
| `lint` | `ubuntu-latest` | Prints that no lint is required. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Outputs | Passing check results with the same job names as `ci.yml`. |
| Permissions | Read-only for the workflow token. |
| Secrets | None. |

## Operational notes

- The jobs are placeholders. They do not build or test anything.
- Keep the paths list in sync with `ci.yml`. A mismatch can leave a required check missing for some changes.

## Sources

- [`.github/workflows/ci-docs.yml`](../ci-docs.yml)
- [`.github/workflows/ci.yml`](../ci.yml)
