# update-dependabot-pr

Regenerates `NOTICE.txt` and `NOTICE-fips.txt` on Dependabot Go module branches and commits the result back to the branch.

## At a glance

| Item | Value |
|------|-------|
| Definition | [`update-dependabot-pr.yml`](../update-dependabot-pr.yml) |
| Triggers | `push` to `dependabot/go_modules/**`, ignoring pushes that only touch `NOTICE.txt` or `NOTICE-fips.txt` |
| Token permissions | `contents: read` (workflow level). `contents: write` at job level. |
| Secrets | None declared. Uses the built-in `GITHUB_TOKEN`. |
| Runner | `ubuntu-latest` |

## Purpose

Dependabot pull requests do not update the notice files, so the lint job fails. This workflow fixes that automatically.

## When it runs

On every push to a `dependabot/go_modules/**` branch that changes files other than the two notice files.

## Jobs and steps

| Job | Steps |
|-----|-------|
| `update-dependabot-pr` | Checks out the branch, sets up Go from `go.mod`, runs `make notice`, detects changes in the notice files, and commits each changed file with the GitHub contents API so the commit is signed. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | The pushed branch and `go.mod`. |
| Outputs | A commit on the Dependabot branch that updates `NOTICE.txt` and `NOTICE-fips.txt`, only when they changed. |
| Permissions | Job token with `contents: write`. |
| Secrets | None declared. |

## Operational notes

- The commit uses `gh api` with the contents endpoint, so GitHub signs it. A local `git commit` would not be signed.
- The step commits `NOTICE.txt` and `NOTICE-fips.txt` as separate API calls, each reading the current blob SHA.
- Source gap: the `make notice` target in the [`Makefile`](../../../Makefile) is not inspected in this pass.

## Sources

- [`.github/workflows/update-dependabot-pr.yml`](../update-dependabot-pr.yml)
- [`Makefile`](../../../Makefile): `notice` target not inspected in this pass.
