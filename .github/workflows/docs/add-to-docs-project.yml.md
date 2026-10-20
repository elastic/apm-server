# add-to-docs-project

Adds an issue to the docs project board when the issue is labeled `Team:Docs`.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/add-to-docs-project.yml`](../add-to-docs-project.yml) |
| Trigger | `issues`, activity type `labeled`; the job runs only when the added label is `Team:Docs` |

## Purpose

Routes documentation requests to the docs project board, using the `Team:Docs` label as the signal. The file does not state further rationale.

## When it runs

- When a label is added to an issue. The job's `if` condition runs it only when `github.event.label.name` is `Team:Docs`.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `add_to_project` | Issue labeled `Team:Docs` | None declared | Runs on `ubuntu-latest`. Creates a GitHub App token with organization projects write and issues read, then runs a GraphQL `addProjectV2ItemById` mutation that adds the issue (`github.event.issue.node_id`) to the hard-coded project node ID in the job `env`. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Issue event payload. |
| Outputs | None. The issue is added to the project. |
| Permissions | Workflow level: `contents: read`. The GitHub App token carries organization projects write and issues read. |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM` (names only). |

## Operational notes

- The target project is hard-coded in the workflow's `env` block. Changing the board requires editing the workflow.
- The GitHub App must be installed with organization projects write access. The installation is not verifiable from this repository.

## Sources

- Workflow: [`.github/workflows/add-to-docs-project.yml`](../add-to-docs-project.yml)
