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

## Smoke tests

Smoke tests are triggered based on a specific scheduler, see [smoke-tests-ess](./smoke-tests-ess.yml) and [smoke-tests-os](./smoke-tests-os.yml)
for further details.

## Bump automation

[updatecli](https://www.updatecli.io/) is the tool we use to automatically bump some of the dependencies related to
the [Elastic Stack versions](./bump-elastic-stack.yml) used for the different tests, the `elastic/Beats.git`
[specific version](./bump-elastic-stack.yml) used by the APM Server or the [Golang bump](./bump-golang.yml) automation.
