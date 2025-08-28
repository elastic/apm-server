# APM Server Release Checklist

The APM Server follows the Elastic Stack release schedule and versions. A release starts with a Feature Freeze period, during which only bug fixes are allowed to be merged into the specific release branch. We generally follow [semver](https://semver.org/) for release versions. For major and minor releases, a new branch is cut from the main branch. For patch releases, only the version on the existing major and minor version branch gets updated. All release workflows (patch, minor, major) has to be triggered manually. The Release Manager will ping the team to align the release process.

This documentation is for 9.x releases. If you are releasing a 8.x look [here](./RELEASES_8x.md)

## Patch Release

1. Create a *Test Plan*.
2. Ensure all relevant backport PRs are merged. We use backport labels on PRs and automation to ensure labels are set.
3. Run the [`run-patch-release`](https://github.com/elastic/apm-server/actions/workflows/run-patch-release.yml) workflow
    - In "Use workflow from", select `main` branch.
    - Then in "The version", specify the **upcoming** patch release version - es: on `8.14.2` feature freeze you will use `8.14.2`).
    - This workflow will:
        - Create the `update-<VERSION>` branch.
        - Update version constants across the codebase and create a PR targeting the release branch.
4. Release notes for patch releases **must be manually added** at least one day before release.
5. Create a PR targeting the `main` branch and add the backport label for the release branch. To add release notes:
    - Add a new section to the existing release notes file ([Sample PR](https://github.com/elastic/apm-server/pull/12680)).
    - Review the [changelogs/head](https://github.com/elastic/apm-server/tree/main/changelogs/head.asciidoc) file and move relevant changelog entries from `head.asciidoc` to `release_version.asciidoc` if the change is backported to release_version. If changes do not apply to the version being released, keep them in the `head.asciidoc` file.
    - Review the commits in the release to ensure all changes are reflected in the release notes. Check for backported changes without release notes in `release_version.asciidoc`.
    - Add your PR to the documentation release issue in the [`elastic/dev`](https://github.com/elastic/dev/issues?q=is%3Aissue%20state%3Aopen%20label%3Adocs) repo ([Sample Issue](https://github.com/elastic/dev/issues/2485)).
    - The PR should be merged the day before release.

## Minor Release

1. Create a *Test Plan*.
2. Run the [`run-minor-release`](https://github.com/elastic/apm-server/actions/workflows/run-minor-release.yml) workflow (In "Use workflow from", select `main` branch. Then in "The version", specify the minor release version the release is for). This workflow will:
    - Create a new release branch using the stack version (X.Y).
    - Update the changelog for the release branch and open a PR targeting the release branch titled `<major>.<minor>: update docs`.
    - Create a PR on `main` titled `<major>.<minor>: update docs, mergify, versions and changelogs`.

## Major Release

1. Create a *Test Plan*.
2. Run the [`run-major-release`](https://github.com/elastic/apm-server/actions/workflows/run-major-release.yml) workflow (In "Use workflow from", select `main` branch. Then in "The version", specify the major release version the release is for). This workflow will:
    - Create a new release branch using the stack version (X.Y).
    - Update the changelog for the release branch and open a PR targeting the release branch titled `<major>.<minor>: update docs`.
    - Create a PR on `main` titled `<major>.0: update docs, mergify, versions and changelogs`.

Before merging them compare commits between latest minor and the new major versions and ensure all relevant PRs have been included in the Changelog. If not, amend it in both PRs. Request and wait a PR review from the team before merging. After it's merged add your PR to the documentation release issue in the [`elastic/dev`](https://github.com/elastic/dev/issues?q=is%3Aissue%20state%3Aopen%20label%3Adocs) repo ([Sample Issue](https://github.com/elastic/dev/issues/2895)).

## Update Dependencies

- libbeat:
    - Updates are automatically created for the release branch, multiple times per week.
    - If there is a need for a manual update, you can run `make update-beats` on the release branch.
    - This might be the case when waiting for an urgent bug fix from beats, or after branching out the release branch.
    - For patch releases, the updates are supposed to only contain bug fixes. Take a quick look at the libbeat changes and raise it with the APM Server team if any larger features or changes are introduced.

- [go-elasticsearch](https://github.com/elastic/go-elasticsearch):
    - If no branch or tag is available, ping the go-elasticsearch team, `go get github.com/elastic/go-elasticsearch/v$major@$major.$minor`.

## Create a Test Plan

Create a [GitHub Issue](https://github.com/elastic/apm-server/issues/new?assignees=&labels=test-plan&projects=&template=test-plan.md) to track testing of the release branch. The issue should include:

- Test all functional changes applied to the new version.
- Any non-functional change or any change already covered by automated tests must not be included.
- Review any PRs updating dependencies, as some functional changes happens through these dependencies.
- Link to PRs in the APM Server repository that need to be tested *manually*.
    - Apply both the `test-plan` label and the appropriate *version label* to the issue - create the version label if it does not already exist.
    - For reference, see the [9.1 Test Plan](https://github.com/elastic/apm-server/issues/17263).
    - For additional examples, you can also view all [previous](https://github.com/elastic/apm-server/issues?q=label%3Atest-plan+is%3Aclosed) test plans.
- Add other test cases that require manual testing, such as test scenarios on ESS, that are not covered by automated tests or OS compatibility smoke tests for supporting new operating systems.

## Between feature freeze and release

- Test the release branch by completing items in the *Test Plan*:
  - Always use a build candidate (BC) when testing, to ensure we test with the distributed artifacts. The first BC is usually available the day after Feature Freeze.
  - Identify which changes require testing via the created test labels, e.g. for [8.3.0](https://github.com/elastic/apm-server/issues?q=label%3Atest-plan+is%3Aclosed+label%3Av8.3.0+-label%3Atest-plan-ok).
  - Grab a PR that hasn't been verified and assign yourself to prevent other devs from re-testing the same change.
  - Test the PR following the Author's how to test this section.
  - Post your testing scenarios on the PR as a comment (for tracking down details in case we run into regressions).
  - Add the `test-plan-ok` or the `test-plan-regression` label to the PR. In case of regression, either open a PR with a fix or open an issue with the details.
- Collaborate with the docs team on any release highlights or breaking changes that should be included in the APM Server guide.

### If a qualifier is needed

- The Release Team will say what qualifier to use in the the `#mission-control` channel.
  - Validate if the qualifier is available for the given branch at `https://storage.googleapis.com/dra-qualifier/<your-branch>`.
  - If so you can trigger a new build manually or wait for a push commit.
  - Otherwise, ask the Robots team as they build an automation to unify the version qualifier for all the observability repositories.

## On release day

* For **minor releases**: new branches need to be added to `conf.yml` in the `elastic/docs` repo. [Example](https://github.com/elastic/docs/pull/893/files#diff-4a701a5adb4359c6abf9b8e1cb38819fR925). **This is handled by the docs release manager.**

* For **patch releases**: if there is an open PR that bumps the version, merge the PR (it may have been created by the GitHub workflow as part of the steps in the ["Day after feature freeze"](#day-after-feature-freeze) section).
  If there is no PR, create one.
 > [!IMPORTANT]
 > Only merge the PRs once pinged on Slack by the Release Manager on release date in the #mission-control channel

* A new [tag](https://github.com/elastic/apm-server/releases) will automatically be created on GitHub.

## When compatibility between Agents & Server changes

* Update the [agent/server compatibility matrix](https://github.com/elastic/observability-docs/blob/main/docs/en/observability/apm/agent-server-compatibility.asciidoc) in the elastic/observability repo.
