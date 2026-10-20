# CI/CD

There are 3 main stages that run on GitHub actions for every push or Pull Request:

* Linting
* Test
* System tests

There are some other stages that run for every push on the main branches:

* [Microbenchmark](./microbenchmark.yml)
* [Benchmarks](./benchmarks.yml)
* Packaging - see below.

## Scenarios

* Tests should be triggered on branch, tag and PR basis.
* Commits that are only affecting the docs files should not trigger any test or similar stages that are not required.
* **This is not the case yet**, but if Github secrets are required then Pull Requests from forked repositories won't run any build accessing those secrets. If needed, then create a feature branch.

## How to interact with the CI?

### On a PR basis

Once a PR has been opened then there are two different ways you can trigger builds in the CI:

1. Commit based.
1. UI based, any Elasticians can force a build through the GitHub UI

### Branches

Every time there is a merge to main or any release branches the main workflow will lint and test all for Linux, Windows and MacOS.

## Package

The packaging and release automation relies on the Unified Release process and Buildkite for generating the
`DRA` packages, for further details please go to [the buildkite folder](../../.buildkite/README.md).

## OpenTelemetry

There is a [GitHub workflow](./opentelemetry.yml) in charge to populate what the workflow run in terms of jobs and steps. Those details can be seen in
[here](https://ela.st/oblt-ci-cd-stats) (**NOTE**: only available for Elasticians).

## Smoke tests OS

Smoke tests are triggered based on a specific scheduler, see [smoke-tests-os](./smoke-tests-os.yml) for further details.

## Bump automation

[updatecli](https://www.updatecli.io/) is the tool we use to automatically bump some of the dependencies related to
the [Elastic Stack versions](./bump-elastic-stack.yml) used for the different tests, the `elastic/Beats.git`
[specific version](./bump-elastic-stack.yml) used by the APM Server or the [Golang bump](./bump-golang.yml) automation.

## Workflow inventory

Each workflow in this directory has a page under [`docs/`](./docs/) with its triggers, jobs, inputs, permissions, secrets (names only), and sources. Composite actions are documented on the pages of the workflows that call them.

| Workflow | Page |
|----------|------|
| `add-to-docs-project.yml` | [add-to-docs-project](./docs/add-to-docs-project.yml.md) |
| `add-to-project.yml` | [add-to-project](./docs/add-to-project.yml.md) |
| `backport-active.yml` | [backport-active](./docs/backport-active.yml.md) |
| `benchmarks-matrix.yml` | [benchmarks-matrix](./docs/benchmarks-matrix.yml.md) |
| `benchmarks.yml` | [benchmarks](./docs/benchmarks.yml.md) |
| `bump-elastic-stack.yml` | [bump-elastic-stack](./docs/bump-elastic-stack.yml.md) |
| `bump-golang.yml` | [bump-golang](./docs/bump-golang.yml.md) |
| `catalog-info.yml` | [catalog-info](./docs/catalog-info.yml.md) |
| `check-docker-compose.yml` | [check-docker-compose](./docs/check-docker-compose.yml.md) |
| `ci-docs.yml` | [ci-docs](./docs/ci-docs.yml.md) |
| `ci.yml` | [ci](./docs/ci.yml.md) |
| `docs-build.yml` | [docs-build](./docs/docs-build.yml.md) |
| `docs-cleanup.yml` | [docs-cleanup](./docs/docs-cleanup.yml.md) |
| `github-commands-comment.yml` | [github-commands-comment](./docs/github-commands-comment.yml.md) |
| `integration-server-test.yml` | [integration-server-test](./docs/integration-server-test.yml.md) |
| `mergify-labels-copier.yml` | [mergify-labels-copier](./docs/mergify-labels-copier.yml.md) |
| `microbenchmark.yml` | [microbenchmark](./docs/microbenchmark.yml.md) |
| `pre-commit.yml` | [pre-commit](./docs/pre-commit.yml.md) |
| `run-major-release.yml` | [run-major-release](./docs/run-major-release.yml.md) |
| `run-minor-release.yml` | [run-minor-release](./docs/run-minor-release.yml.md) |
| `run-patch-release.yml` | [run-patch-release](./docs/run-patch-release.yml.md) |
| `smoke-tests-os-sched.yml` | [smoke-tests-os-sched](./docs/smoke-tests-os-sched.yml.md) |
| `smoke-tests-os.yml` | [smoke-tests-os](./docs/smoke-tests-os.yml.md) |
| `terraform-fmt.yml` | [terraform-fmt](./docs/terraform-fmt.yml.md) |
| `update-beats.yml` | [update-beats](./docs/update-beats.yml.md) |
| `update-compose.yml` | [update-compose](./docs/update-compose.yml.md) |
| `update-dependabot-pr.yml` | [update-dependabot-pr](./docs/update-dependabot-pr.yml.md) |

The existing `./opentelemetry.yml` link above points to a file that is not in this directory. It is kept as written; it is a known gap.

