---
description: GitHub issue format for updatecli workflow failures in apm-server
applyTo: "**/*"
---

Create one GitHub issue with the title:
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

Follow this format exactly, keep the scope to this repository only, and do not re-run,
cancel or modify any workflow run, or close or merge pull requests.
