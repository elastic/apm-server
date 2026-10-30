---
on:
  # Fuzzy weekly schedule distributes execution time and reduces load spikes.
  # A weekly report builds a history of failures for the host of the week.
  schedule:
    - cron: weekly on monday
  # Can also be triggered on demand to investigate an active outage.
  workflow_dispatch:
    inputs:
      lookback_days:
        type: string
        default: "7"
        description: "Days of run history to inspect"

# Each on-demand dispatch gets its own concurrency slot so two triggered runs
# don't cancel each other; scheduled runs share a single slot (deduplication).
concurrency:
  job-discriminator: "${{ github.run_id }}"

permissions:
  actions: read
  contents: read
  issues: read
  pull-requests: read
  copilot-requests: write

# Load the debug-updatecli skill from this repo via the vendored APM workflow.
# The compiler adds an `apm` job that installs the skill bundle; the agent
# picks it up via progressive disclosure at runtime.
imports:
  - uses: shared/apm.md
    with:
      target: copilot
      packages:
        - elastic/apm-server/.apm/skills/debug-updatecli
        - elastic/apm-server/.apm/instructions/updatecli-failure-issue.instructions.md

# Run logs are read with the pre-authenticated gh CLI; no extra secrets needed.
tools:
  github:
    mode: gh-proxy
    toolsets: [default, actions]

# `gh run view --log-failed` is redirected to the Actions log storage domains.
network:
  allowed:
    - defaults
    - github-actions

safe-outputs:
  create-issue:
    title-prefix: "[updatecli] "
    labels: [automation, ci]
    max: 1
    # One report per week: a new one supersedes the previous.

---

# updatecli-failure-report

Use the **debug-updatecli** skill to investigate the failed runs of the updatecli
workflows (`bump-elastic-stack-snapshot`, `bump-golang`, `update-beats`, `update-compose`)
on `main` in `${{ github.repository }}` over the last `${{ inputs.lookback_days || '7' }}` days.
Group the failures by workflow and pattern, and check recurrence in the same window.
If nothing failed, do not create an issue. Follow the skill and the
`.apm/instructions/updatecli-failure-issue.instructions.md` output requirements.
