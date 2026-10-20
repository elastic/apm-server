# apm-server-package (Buildkite)

Builds the APM Server packages for snapshot and staging channels on x86-64 and aarch64, then runs the DRA step that publishes them. It runs on commits and on `/package` pull request comments, as configured in `pull-requests.json`.

## At a glance

| Item | Value |
|------|-------|
| Pipeline definition | [`package.yml`](../package.yml) |
| Pipeline slug | `apm-server-package` |
| Trigger config | [`pull-requests.json`](../pull-requests.json) |
| Triggers | Every commit (`build_on_commit`); pull request comments matching `^/package` (`build_on_comment`) |
| Skip rules | Label `skip-ci`; target branches: none; changes only in listed paths |
| Access | Org users, repository permission `admin` or `write`, plus four bot accounts (`allowed_list`) |
| Hooks | [`pre-command`](../hooks/pre-command), [`pre-exit`](../hooks/pre-exit) |
| Secrets | Loaded by the `pre-command` hook from Vault (names not verified in this pass) |

## Purpose

Builds the APM Server distributions and publishes them to the Unified Release (DRA) process, for both the snapshot and staging channels.

## When it runs

| Event | Behavior |
|-------|----------|
| Commit pushed to a pull request | Runs when the commit is not skipped by a rule below. |
| Comment `/package` on a pull request | Runs the build even when a commit-based build would be skipped. |
| Skip label `skip-ci` | Skips commit-based builds. |
| Only listed paths changed | Skips commit-based builds. The listed paths are `.ci/`, `.github/`, `catalog-info.yaml`, `updatecli-compose.yaml`, `changelog*`, `docs/`, any `.md` file, `docker-compose.yml`, `.mergify.yml`, `.pre-commit-config.yaml`, any `.txt` file, and `Tiltfile`. |

The regex `^/package` matches a comment that starts with `/package`. The `always_trigger_comment_regex` setting has the same value, so the comment runs the build even when skip rules apply.

`allow_org_users` is `true`, and `allowed_repo_permissions` is `admin` and `write`. Users with other roles can trigger builds only if their account is in `allowed_list`. Whether this applies to fork pull requests is a source gap.

## Pipeline steps

| Step | Agent | Matrix | Command | Artifacts |
|------|-------|--------|---------|-----------|
| Package x86_64 | GCP, `c2-standard-16`, Ubuntu 24.04 | `snapshot`, `staging` | `.buildkite/scripts/package.sh {{matrix}}` | `build/distributions/**/*`, `build/dependencies*.csv` |
| Package aarch64 | AWS, `t4g.2xlarge`, Ubuntu 24.04 ARM | `snapshot`, `staging` | `.buildkite/scripts/package.sh {{matrix}}` | `build/distributions/**/*` |
| DRA | GCP, `c2-standard-16`, Ubuntu 24.04 | `snapshot`, `staging` | `.buildkite/scripts/dra.sh {{matrix}}` | None |

- The package steps retry automatically once.
- The DRA step depends on the `package` group and does not allow failure of that dependency.
- The pipeline sets `GENERATE_WOLFI_IMAGES` and `GENERATE_FIPS_ARTIFACTS` to `true`.
- The `notify` block posts a Slack message to `#apm-server` and `#observablt-bots` when a build fails on `main`, a `MAJOR.MINOR` branch, or a `MAJOR.x` branch.

## Inputs, outputs, and access

| Area | Details |
|------|---------|
| Inputs | Commit SHA and branch from Buildkite. The matrix values `snapshot` and `staging` select the channel. |
| Outputs | Package distributions and dependency CSVs as Buildkite artifacts. The DRA step publishes them through the Unified Release process. |
| Permissions | Comment triggers need the repository permission and account rules above. |
| Secrets | Loaded from Vault in `pre-command`. Names are not listed here because the hook was not fully read. This is a source gap. |

## Dynamic uploads

The pipeline's `package.sh` and `dra.sh` scripts were read only in part. They may upload additional steps or artifacts at runtime. This is a source gap, and the list of uploaded steps is not documented here.

## Operational notes

- The `pre-command` hook skips upload commands and loads Vault secrets. The `pre-exit` hook cleans up.
- The package README in the repository root says builds run on merged pull requests. The current `pull-requests.json` triggers on commits and `/package` comments. The README is older than the trigger configuration, and this page describes the configuration.
- The `dra.sh` and `package.sh` scripts, the hooks, and the `catalog-info.yaml` file were only partly reviewed. These are source gaps.

## Sources

- [`.buildkite/package.yml`](../package.yml)
- [`.buildkite/pull-requests.json`](../pull-requests.json)
- [`.buildkite/hooks/pre-command`](../hooks/pre-command)
- [`.buildkite/hooks/pre-exit`](../hooks/pre-exit)
- [`.buildkite/scripts/package.sh`](../scripts/package.sh) (partly read)
- [`.buildkite/scripts/dra.sh`](../scripts/dra.sh) (partly read)
- [`.buildkite/scripts/utils.sh`](../scripts/utils.sh)
- Source gaps: full `package.sh` and `dra.sh` logic, the `pre-command` Vault secret names, the Buildkite pipeline settings outside this repository, and fork pull request handling.
