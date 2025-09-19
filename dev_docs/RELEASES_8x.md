# Release 8.x

We expect no more minor versions in the `8.x` line after 8.19, but only patch releases.

## 8.17 patch releases

Run the `run-patch-release` with any `8.17.x` version.

**NOTE**: This automation does not handle the changelog, which should be manually handled.

## 8.18 patch releases

Run the `run-patch-release` with any `8.18.x` version.

**NOTE**: This automation does not handle the changelog, which should be manually handled.

## 8.19.0

This is technically a minor release, but it's the last minor we will ever create on the 8.x line.
The trunk branch `8.x` does not exist anymore (as expected) and has been renamed to `8.19`. This
makes our current minor release automation broken.

The `run-minor-release` automation is currently not working, and fixing it is not worth for a single release.
As such for this release we should do it manually.

What does that include?
- update the changelog in `8.19` branch
- update versions to `8.19.1`

The process:
- switch to `8.19` branch
- create a new branch: `release-8.19.0`
- update the changelog, manually
- update versions with `make update-version VERSION=8.19.1`
- push the branch and create a Pull Request named: `8.19.0: update changelog, versions`

An example PR for the changelog update can be seen here: https://github.com/elastic/apm-server/pull/14382/files  
An example PR for the other changes can be seen here, but note that this time `8.x` branch is `8.19`: https://github.com/elastic/apm-server/pull/14381/files

Refer to `release.mk` and the `minor-release` target for further details.

## 8.19 patch releases

Run the `run-patch-release` with any `8.19.x` version.

**NOTE**: This automation does not handle the changelog, which should be manually handled.


## Feature Freeze

* For **patch releases**:
  * ensure all relevant backport PRs are merged. We use backport labels on PRs and automation to ensure labels are set.


## Day after Feature Freeze

* Trigger release workflow manually
  * For **patch releases**: run the [`run-patch-release`](https://github.com/elastic/apm-server/actions/workflows/run-patch-release.yml) workflow. In "Use workflow from", specify the following values:
       * Branch: Select the relevant `8.x` branch - e.g: `8.14` for `8.14.x` patch releases.
       * Version: Specify the **upcoming** patch release version - e.g: on `8.14.2` feature freeze you will use `8.14.2`.
   
    This workflow will: create the `update-<VERSION>` branch, update version constants across the codebase and create a PR targeting the release branch.
    
    Release notes for patch releases **must be manually added** at least one day before release.
    Create a PR targeting the relevant `8.x` branch.
    To add release notes:
    * Add a new section to the existing release notes file ([Sample PR](https://github.com/elastic/apm-server/pull/12680)).
    * Review the [changelogs/head](https://github.com/elastic/apm-server/tree/main/changelogs/head.asciidoc) file and move relevant changelog entries from `head.asciidoc` to `release_version.asciidoc` if the change is backported to release_version. If changes do not apply to the version being released, keep them in the `head.asciidoc` file.
    * Review the commits in the release to ensure all changes are reflected in the release notes. Check for backported changes without release notes in `release_version.asciidoc`.
    * Add your PR to the documentation release issue in the [`elastic/dev`](https://github.com/elastic/dev/issues?q=is%3Aissue%20state%3Aopen%20label%3Adocs) repo ([Sample Issue](https://github.com/elastic/dev/issues/2485)).
    * The PR should be merged the day before release.
  * For **minor releases**: run the [`run-minor-release`](https://github.com/elastic/apm-server/actions/workflows/run-minor-release.yml) workflow (In "Use workflow from", select `main` branch. Then in "The version", specify the minor release version the release is for).
    This workflow will: create a new release branch using the stack version (X.Y); update the changelog for the release branch and open a PR targeting the release branch titled `<major>.<minor>: update docs`; create a PR on `main` titled `<major>.<minor>: update docs, mergify, versions and changelogs`. Before merging them compare commits between latest minor and the new minor versions and ensure all relevant PRs have been included in the Changelog. If not, amend it in both PRs. Request and wait a PR review from the team before merging. After it's merged add your PR to the documentation release issue in the [`elastic/dev`](https://github.com/elastic/dev/issues?q=is%3Aissue%20state%3Aopen%20label%3Adocs) repo ([Sample Issue](https://github.com/elastic/dev/issues/2895)).
  * For **major releases**: run the [`run-major-release`](https://github.com/elastic/apm-server/actions/workflows/run-major-release.yml) workflow (In "Use workflow from", select `main` branch. Then in "The version", specify the major release version the release is for).
    This workflow will: create a new release branch using the stack version (X.Y); update the changelog for the release branch and open a PR targeting the release branch titled `<major>.<minor>: update docs`; create a PR on `main` titled `<major>.0: update docs, mergify, versions and changelogs`. Before merging them compare commits between latest minor and the new major versions and ensure all relevant PRs have been included in the Changelog. If not, amend it in both PRs. Request and wait a PR review from the team before merging. After it's merged add your PR to the documentation release issue in the [`elastic/dev`](https://github.com/elastic/dev/issues?q=is%3Aissue%20state%3Aopen%20label%3Adocs) repo ([Sample Issue](https://github.com/elastic/dev/issues/2895)).
* The Release Manager will ping the team to align the release process

* Update dependencies

  * libbeat:
    Updates are automatically created for the release branch, multiple times per week.
    If there is a need for a manual update, you can run `make update-beats` on the release branch.
    This might be the case when waiting for an urgent bug fix from beats, or after branching out the release branch.

    For patch releases, the updates are supposed to only contain bug fixes. Take a quick look at the libbeat changes
    and raise it with the APM Server team if any larger features or changes are introduced.

  * [go-elasticsearch](https://github.com/elastic/go-elasticsearch):
    If no branch or tag is available, ping the go-elasticsearch team.

    `go get github.com/elastic/go-elasticsearch/v$major@$major.$minor`

* Test plan

  Create a github issue for testing the release branch ([use the GitHub issue `test plan` template](https://github.com/elastic/apm-server/issues/new?assignees=&labels=test-plan&projects=&template=test-plan.md)), It should contain:
  * A link to all PRs in the APM Server repository that need to be tested manually to create an overview over the PRs that need testing.
    Use the `test-plan` label and the version label (create it if it does not exist). For example, [this was 8.13.0 test plan](https://github.com/elastic/apm-server/issues/12822)
    and here you can find [all previous test plans](https://github.com/elastic/apm-server/issues?q=label%3Atest-plan+is%3Aclosed).
    What we aim for is testing all functional changes applied to the new version. Review any PR updating `elastic/go-docappender` and `elastic/apm-data` dependencies, as some functional changes happens through these dependencies.
   Any non-functional change or any change already covered by automated tests must not be included.
  * Add other test cases that require manual testing, such as test scenarios on ESS, that are not covered by automated tests or OS compatibility smoke tests for supporting new operating systems.

## Between feature freeze and release

* Test the release branch

  * Always use a build candidate (BC) when testing, to ensure we test with the distributed artifacts. The first BC is usually available the day after Feature Freeze.
  * Identify which changes require testing via the created test labels, e.g. for [8.3.0](https://github.com/elastic/apm-server/issues?q=label%3Atest-plan+is%3Aclosed+label%3Av8.3.0+-label%3Atest-plan-ok).
  * Grab a PR that hasn't been verified and assign yourself to prevent other devs from re-testing the same change.
  * Test the PR following the Author's how to test this section.
  * Post your testing scenarios on the PR as a comment (for tracking down details in case we run into regressions).
  * Add the `test-plan-ok` or the `test-plan-regression` label to the PR. In case of regression, either open a PR with a fix or open an issue with the details.

* Collaborate with the docs team on any release highlights or breaking changes that should be included in the APM Server guide.

* Run DRA for a given qualifier. The Release Team will say what qualifier to use in the the #mission-control channel.
  * Go to https://buildkite.com/elastic/apm-server-package
  * Click on `New Build`.
  * Choose the `Branch` where the release should come from (either `main`, `8.x` or `[0-9].[0-9]+)`_
  * Click on `options`
  * Add `ELASTIC_QUALIFIER=<qualifier>` (`<qualifier` should be replaced with the given qualifier)
  * Click on `Create Build`.

## On release day

* For **minor releases**: new branches need to be added to `conf.yml` in the `elastic/docs` repo. [Example](https://github.com/elastic/docs/pull/893/files#diff-4a701a5adb4359c6abf9b8e1cb38819fR925). **This is handled by the docs release manager.**

* For **patch releases**: if there is an open PR that bumps the version, merge the PR (it may have been created by the GitHub workflow as part of the steps in the ["Day after feature freeze"](#day-after-feature-freeze) section).
  If there is no PR, create one.
 > [!IMPORTANT]
 > Only merge the PRs once pinged on Slack by the Release Manager on release date in the #mission-control channel

* A new [tag](https://github.com/elastic/apm-server/releases) will automatically be created on GitHub.

## When compatibility between Agents & Server changes

* Update the [agent/server compatibility matrix](https://github.com/elastic/observability-docs/blob/main/docs/en/observability/apm/agent-server-compatibility.asciidoc) in the elastic/observability repo.

## Templates

Templates for adding release notes, breaking changes, and highlights.

<details><summary><code>/changelogs/*.asciidoc</code> template</summary>

```asciidoc
[[apm-release-notes-8.1]]
== APM Server version 8.1

https://github.com/elastic/apm-server/compare/8.0\...8.1[View commits]

* <<apm-release-notes-8.1.0>>

[[apm-release-notes-8.1.0]]
=== APM Server version 8.1.0

https://github.com/elastic/apm-server/compare/v8.0.1\...v8.1.0[View commits]

No significant changes.
////
[float]
==== Breaking Changes

[float]
==== Bug fixes

[float]
==== Intake API Changes

[float]
==== Added
////
```
</details>

<details><summary><code>apm-release-notes.asciidoc</code> template</summary>

```asciidoc
* <<release-highlights-8.1.0>>

[[release-highlights-8.1.0]]
=== APM version 8.1.0

No new features
////
[float]
==== New features

* Feature name and explanation...
////
```
</details>

<details><summary><code>apm-breaking-changes.asciidoc</code> template</summary>

```asciidoc
* <<breaking-8.0.0, APM version 8.0.0>>

[[breaking-8.0.0]]
=== Breaking changes in 8.0.0

APM Server::
+
[[slug]]
**Title** Topic...

APM UI::
+
[[slug]]
**Title** Topic...
```
</details>

