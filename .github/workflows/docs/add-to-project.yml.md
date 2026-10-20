# add-to-project

Adds newly opened or transferred issues to the team project board.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`add-to-project.yml`](../add-to-project.yml) |
| Triggers | `issues`: `opened`, `transferred` |
| Token permissions | `contents: read` (workflow level) |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM` |
| Runner | `ubuntu-latest` |

## Purpose

Keeps the team project board in sync with new issues without manual triage.

## When it runs

Runs for each issue that is opened or transferred into this repository.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `add-to-project` | Creates a GitHub App token with organization projects write and issues read, then adds the issue to the project with `actions/add-to-project`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Issue event payload only. |
| Outputs | None. The issue is added to the project. |
| Permissions | Job token from the GitHub App. The workflow token is read-only. |
| Secrets | Names only: `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`. |

## Operational notes

- The target project is hard-coded in the workflow. Changing the board requires editing the workflow.
- The GitHub App must be installed with organization projects write access.

## Sources

- [`.github/workflows/add-to-project.yml`](../add-to-project.yml)
