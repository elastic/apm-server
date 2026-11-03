# add-to-project

Adds newly opened or transferred issues to the team project board.

## At a glance

| | |
| --- | --- |
| Platform | GitHub Actions |
| Source | [`.github/workflows/add-to-project.yml`](../add-to-project.yml) |
| Trigger | `issues`, activity types `opened`, `transferred` |

## Purpose

Adds new issues to the team project board so they are tracked without manual triage. The file does not state further rationale.

## When it runs

- Each time an issue is opened or transferred into this repository. The file declares no label or actor filters and no job condition.

## Execution flow

| Job | Runs when | Depends on | What it does |
| --- | --- | --- | --- |
| `add-to-project` | Issue opened or transferred | None declared | Runs on `ubuntu-latest`. Creates a GitHub App token with organization projects write and issues read, then runs `actions/add-to-project` against the hard-coded project URL using that token. |

## Inputs, outputs, and access

| Item | Details |
| --- | --- |
| Inputs | Issue event payload. |
| Outputs | None. The issue is added to the project. |
| Permissions | Workflow level: `contents: read`. The job uses a GitHub App token with organization projects write and issues read. |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM` (names only). |

## Operational notes

- The target project is hard-coded in the workflow. Changing the board requires editing the workflow.
- The GitHub App must be installed with organization projects write access. The installation is not verifiable from this repository.

## Sources

- Workflow: [`.github/workflows/add-to-project.yml`](../add-to-project.yml)
