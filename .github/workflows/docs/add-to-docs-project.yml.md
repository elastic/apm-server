# add-to-docs-project

Adds an issue to the docs project board when it receives the `Team:Docs` label.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`add-to-docs-project.yml`](../add-to-docs-project.yml) |
| Triggers | `issues`: `labeled` (runs only when the label is `Team:Docs`) |
| Token permissions | `contents: read` (workflow level) |
| Secrets | `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM` |
| Runner | `ubuntu-latest` |

## Purpose

Routes documentation requests to the docs project board using the `Team:Docs` label as the signal.

## When it runs

Runs when any label is added to an issue. The job's `if` condition skips every label except `Team:Docs`.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `add_to_project` | Creates a GitHub App token, then runs a GraphQL `addProjectV2ItemById` mutation with the issue node ID and a hard-coded project ID. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Issue event payload only. |
| Outputs | None. |
| Permissions | GitHub App token with organization projects write and issues read. |
| Secrets | Names only: `OBS_AUTOMATION_APP_ID`, `OBS_AUTOMATION_APP_PEM`. |

## Operational notes

- The project ID is hard-coded in the workflow's `env` block.

## Sources

- [`.github/workflows/add-to-docs-project.yml`](../add-to-docs-project.yml)
