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
`${{ github.event.workflow_run.id || inputs.run_id }}` in `${{ github.repository }}`
and check recurrence of the same workflow over the last
`${{ inputs.lookback_days || '14' }}` days. Follow the skill and the
`.apm/instructions/updatecli-failure-issue.instructions.md` output requirements.
