---
on:
  # Triage every failed updatecli automation run.
  workflow_run:
    workflows:
      - bump-elastic-stack-snapshot
      - bump-golang
      - update-beats
      - update-compose
    types: [completed]
    branches: [main]
    conclusion: [failure, timed_out]
  # Can also be triggered on demand to investigate a specific run.
  workflow_dispatch:
    inputs:
      run_id:
        type: string
        required: true
        description: "ID of the failed workflow run to analyse"
      lookback_days:
        type: string
        default: "14"
        description: "Days of run history to inspect for recurrence"

# One slot per analysed run: failures of different runs don't cancel each other.
concurrency:
  job-discriminator: "${{ github.event.workflow_run.id || inputs.run_id }}"

permissions:
  actions: read
  contents: read
  issues: read
  pull-requests: read
  copilot-requests: write

# Load the debug-updatecli skill from this repo via APM.
# The compiler adds an `apm` job that installs the skill bundle; the agent
# picks it up via progressive disclosure at runtime.
imports:
  - uses: microsoft/apm/.github/workflows/shared/apm.md@v0.32.0
    with:
      target: copilot
      packages:
        - elastic/apm-server/.skills/debug-updatecli

# Run logs are read with the pre-authenticated gh CLI; no extra secrets needed.
tools:
  github:
    mode: gh-proxy
    toolsets: [default, actions]

# `gh run view --log-failed` is redirected to the Actions log storage domains.
network:
  allowed:
    - defaults
    - github
    - github-actions

safe-outputs:
  create-issue:
    title-prefix: "[updatecli] "
    labels: [automation, ci]
    max: 1
    # Only supersede older issues of the same upstream workflow.
    close-older-issues: true
    close-older-key: "${{ github.event.workflow_run.name || 'manual' }}"

---

# updatecli-failure-report

Use the **debug-updatecli** skill to investigate the failed workflow run
`${{ github.event.workflow_run.id || inputs.run_id }}` in `${{ github.repository }}`.
The skill is pre-installed via APM; activate it when you start.

## Instructions

1. Follow the debug-updatecli skill to classify the failure of that run, then check
   recurrence of the same workflow over the last `${{ inputs.lookback_days || '14' }}` days.

2. Create one GitHub issue with the title:
   `<workflow name> failed on <branches> — <pattern> (N failures in last D days)`

   The issue must include:

   **Summary** — one paragraph: which workflow and branches failed, the root cause, and
   whether it is transient or needs action.

   **Failure frequency** — table with one row per run over the lookback window (pass and fail):

   | Date | Run | Branches failed | Conclusion | Pattern |
   |------|-----|-----------------|------------|---------|

   **Root cause analysis** — the exact error lines (in a code block), the stage that failed
   (source / condition / target / action), and why it broke now.

   **Timeline** — first failure in the window; correlation with events such as a new or retired
   branch, a change to the updatecli config or workflow, an updatecli release or a GitHub incident.

   **Recommended fix** — exact files and line references, what to change and trade-offs.
   If the failure is transient and the next run recovered, say that no change is needed.

   **Links** — the analysed run, the three most recent failing runs, and any related PR.

## Notes

- Keep the scope to this repository only.
- Do not re-run, cancel or modify any workflow run, and do not close or merge pull requests.
