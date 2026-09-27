---
name: debug-updatecli
description: >
  Diagnose failing updatecli GitHub Actions workflows in apm-server
  (bump-elastic-stack-snapshot, bump-golang, update-beats, update-compose).
  Fetches failed runs, reads job logs, classifies root causes and checks whether
  they recur. Use when an updatecli automation workflow fails.
compatibility: Requires the gh CLI authenticated with actions:read on elastic/apm-server.
metadata:
  author: elastic
  repository: elastic/apm-server
---

# Debug updatecli workflow failures

Investigate the workflow run provided by the user. If no run is given, start from the
most recent failed run of the workflows below.

## Workflows in scope

| Workflow name | File | updatecli config |
|---------------|------|------------------|
| `bump-elastic-stack-snapshot` | `.github/workflows/bump-elastic-stack.yml` | `.ci/updatecli/bump-elastic-stack-snapshot.yml` |
| `bump-golang` | `.github/workflows/bump-golang.yml` | `.ci/updatecli/bump-golang.yml` |
| `update-beats` | `.github/workflows/update-beats.yml` | `.ci/updatecli/update-beats.yml` |
| `update-compose` | `.github/workflows/update-compose.yml` | `updatecli-compose.yaml` (policies, values in `.ci/updatecli/values.d/updatecli-compose.yml`) |

All of them run `elastic/oblt-actions/updatecli/run@v1`, some of them in a matrix over the
active branches (`main`, `9.x`, `8.19`, ...). Shared SCM values live in
`.ci/updatecli/values.d/scm.yml`.

## Step 1 — Inspect the failed run

```bash
gh run view <run_id> --repo elastic/apm-server --json name,conclusion,createdAt,event,jobs,url
gh run view <run_id> --repo elastic/apm-server --log-failed
```

Note which matrix jobs (branches) failed and which step failed. Logs can be long; focus on
lines containing `ERROR`, `##[error]`, `✗` and the `SUMMARY` block updatecli prints at the end.

## Step 2 — Classify the failure

### Pattern A — pull request already exists
```
ERROR: action stage: "creating pull request: A pull request already exists for elastic:updatecli_<branch>_<pipelineid>."
```
updatecli pushed the branch but failed to find or reuse the open PR. Check the PR mentioned in
`Existing GitHub pull request found:`:

```bash
gh pr view <number> --repo elastic/apm-server --json state,createdAt,headRefName,mergeable
```

- PR `createdAt` within seconds of the failing step → race / retry in the GitHub API during PR
  creation. The PR is fine; the failure is cosmetic.
- PR is old and still open → the automation PR is stale (not merged, failing CI, or conflicts).
  Report why it is stuck.

### Pattern B — GitHub API transient error
Log contains `429 (Too Many Requests)`, `503 Service Unavailable`, `unable to query GitHub API rate limit`,
`secondary rate limit` or `Failed to download archive ... after 3 attempts`.
Transient. Check whether the next scheduled run succeeded.

### Pattern C — source not available
Failure in a `source` stage, e.g. the snapshot JSON at
`https://storage.googleapis.com/artifacts-api/snapshots/<branch>.json` returns 404 or an empty
`build_id`, or a Go / Beats version cannot be resolved. Usually the branch was just created or
retired: compare against the active branches matrix.

### Pattern D — updatecli configuration / version error
Template errors, `requiredEnv` missing, unknown resource kind, or deprecated engine
(e.g. `Engine "dasel/v1" is deprecated`). Read the updatecli config file for the workflow and
point to the exact line to change.

### Pattern E — target / condition failure
A `target` or `condition` stage fails (file not found, yaml key missing, regex does not match).
The target files likely moved or changed format on that branch. Read the file on that branch:

```bash
gh api -H "Accept: application/vnd.github.raw" "repos/elastic/apm-server/contents/<path>?ref=<branch>"
```

### Pattern F — authentication
`Bad credentials`, `Resource not accessible by integration`, or the `Get token` step fails.
The GitHub App token (`OBS_AUTOMATION_APP_*`) is missing permissions or expired.

Anything else: quote the relevant log lines and describe it as unclassified.

## Step 3 — Check recurrence

List recent runs of the same workflow (default lookback: 14 days):

```bash
gh run list --repo elastic/apm-server --workflow <file> --limit 50 \
  --json databaseId,conclusion,createdAt,url
```

For every failed run in the window, fetch `--log-failed` and classify it with Step 2. Build a
timeline: first occurrence, how many runs failed with the same pattern, which branches, and
whether the next run recovered on its own. Logs older than the retention period may be gone —
say so rather than guessing.

## Step 4 — Summarise and propose a fix

Report:
1. Which workflow, run and branches failed, and since when
2. The exact error lines
3. The pattern (A–F or unclassified) and whether it is transient or needs action
4. Whether it is recurring, with the frequency over the lookback window
5. The specific file(s) and lines to change and what to change, or why no change is needed
6. Any risk or trade-off of the fix

Do not re-run, cancel or modify workflow runs, and do not close or merge pull requests.
