# update-dependabot-pr

Regenerates `NOTICE.txt` and `NOTICE-fips.txt` on Dependabot Go module branches and pushes any changes back to the branch.

## At a glance

| Item | Value |
|------|-------|
| Platform | GitHub Actions |
| Source | [`update-dependabot-pr.yml`](../update-dependabot-pr.yml) |
| Trigger | `push` to `dependabot/go_modules/**`, excluding pushes that touch only the notice files |
| Runner | `ubuntu-latest` |

## Purpose

Dependabot pull requests do not update the notice files, which makes the lint job fail. This workflow regenerates the notices and pushes the result to the Dependabot branch.

## When it runs

- On every push to a `dependabot/go_modules/**` branch, unless every changed file is `NOTICE.txt` or `NOTICE-fips.txt`.

## Execution flow

| Job | Runs when | Depends on | What it does |
|-----|-----------|------------|--------------|
| `update-dependabot-pr` | Every matching push | None | Gets a token with `elastic/oblt-actions/github/create-token@v1`, checks out the branch with that token, configures git with `elastic/oblt-actions/git/setup@v1`, imports a GPG key, sets up Go from `go.mod`, and runs `make notice`. If `git status` shows changes to `NOTICE.txt` or `NOTICE-fips.txt`, it runs `git add`, `git commit -m "Update NOTICE.txt and NOTICE-fips.txt"`, and `git push`. |

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | The pushed Dependabot branch and `go.mod`. |
| Outputs | A commit on the Dependabot branch that updates `NOTICE.txt` and `NOTICE-fips.txt`, only when they changed. |
| Permissions | Workflow token `contents: read`. The job requests `contents: read` and `id-token: write`. The checkout and push use the token from `create-token`. |
| Secrets | `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` (names only). |

## Operational notes

- The GPG import step enables commit signing (`git_commit_gpgsign: true`), so the commit is signed without a `-S` flag.
- `make notice` runs `go-licence-detector` against the modules in `go.mod`, using `tools/notice/overrides.json` and `tools/notice/rules.json` (see [`Makefile`](../../../Makefile)).
- The token policy for `create-token` is defined in an external catalog repository, linked from the workflow comments. It is not verified here.
- Source gap: the `elastic/oblt-actions` actions, `crazy-max/ghaction-import-gpg`, and `actions/setup-go` are external. Their behavior is not verified here.

## Sources

- [`.github/workflows/update-dependabot-pr.yml`](../update-dependabot-pr.yml)
- [`Makefile`](../../../Makefile) (`notice` target)
- [`tools/notice/overrides.json`](../../../tools/notice/overrides.json)
- [`tools/notice/rules.json`](../../../tools/notice/rules.json)
