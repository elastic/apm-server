---
name: Test Plan
about: Create a new manual test plan meta issue
labels: "test-plan"

---

# Manual Test Plan

When picking up a test case, please add your name to this overview beforehand and tick the checkbox when finished.
Testing can be started when the first build candidate (BC) is available in the CFT region.

## Smoke Testing ESS setup

Thanks to https://github.com/elastic/apm-server/issues/8303 further smoke tests are run automatically on ESS now.
**Consider extending the smoke tests to include more test cases which we'd like to cover**

## Go agent

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-agent-go/pulls-->

## Lambda extension

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-aws-lambda/pulls-->

## APM Kubernetes Attacher

<!-- Add any issues / PRs which were worked on during the milestone release -->

## Test cases from the GitHub board

<!-- Replace MAJOR.MINOR with the appropriate versions below -->
<!-- Label the relevant MAJOR.MINOR Issues / PRs with the `test-plan` label https://github.com/elastic/apm-server/issues?page=1&q=-label%3Atest-plan+label%3AvMAJOR.MINOR.0+-label%3Atest-plan-ok-->
<!-- [apm-server MAJOR.MINOR test-plan](https://github.com/elastic/apm-server/issues?q=is%3Aissue+label%3Atest-plan+-label%3Atest-plan-ok+is%3Aclosed+label%3AvMAJOR.MINOR.0) -->

Add yourself as _assignee_ on the PR before you start testing.

## Regressions

Link any regressions to this issue.
