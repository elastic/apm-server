# APM Server release checklist

The APM Server follows the Elastic Stack release schedule and versions.
A release starts with a Feature Freeze period, during which only bug fixes
are allowed to be merged into the specific release branch.
We generally follow [semver](https://semver.org/) for release versions.
For major and minor releases a new branch is cut off the main branch.
For patch releases, only the version on the existing major and minor version branch is updated.

## Feature Freeze

* For **patch releases**:
  * ensure all relevant backport PRs are merged. We use backport labels on PRs and automation to ensure labels are set.


## Day after Feature Freeze

* Trigger release workflow manually
  * For **patch releases**: run the [`run-patch-release`](https://github.com/elastic/apm-server/actions/workflows/run-patch-release.yml) workflow (In "Use workflow from", select `main` branch. Then in "The version", specify the **upcoming** patch release version - es: on `8.14.2` feature freeze you will use `8.14.2`).
    This workflow will: create the release branch; update version across codebase; commit and create PR targeting the release branch.
    Release notes for patch releases **must be manually added** (PR should target `main` branch and backported to the release branch):
    * Add a new section to the existing release notes file ([Sample PR](https://github.com/elastic/apm-server/pull/12680)).
    * Review the [changelogs/head](https://github.com/elastic/apm-server/tree/main/changelogs/head.asciidoc) file and move relevant changelog entries from `head.asciidoc` to `release_version.asciidoc` if the change is backported to release_version. If changes do not apply to the version being released, keep them in the `head.asciidoc` file.
    * Review the commits in the release to ensure all changes are reflected in the release notes. Check for backported changes without release notes in `release_version.asciidoc`.
    * Add your PR to the documentation release issue ([Sample Issue](https://github.com/elastic/dev/issues/2485)).
    * The PR should be merged the day before release.
  * For **minor releases**: run the [`run-minor-release`](https://github.com/elastic/apm-server/actions/workflows/run-minor-release.yml) workflow (In "Use workflow from", select `main` branch. Then in "The version", specify the minor release version the release is for).  
    This workflow will: create the release branch; update the changelog for the release branch and open a PR targeting the release branch titled `<major>.<minor>: update docs`; create a PR on `main` titled `<major>.<minor>: update docs, mergify, versions and changelogs`. Before merging them compare commits between latest minor and the new minor versions and ensure all relevant PRs have been included in the Changelog. If not, amend it in both PRs. Request and wait a PR review from the team before merging.
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
   Any non-functional change or any change that is already covered by automated tests must not be included.
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

## On release day

* New branches need to be added to `conf.yml` in the `elastic/docs` repo. [Example](https://github.com/elastic/docs/pull/893/files#diff-4a701a5adb4359c6abf9b8e1cb38819fR925). **This is handled by the docs release manager.**

* A new [tag](https://github.com/elastic/apm-server/releases) will automatically be created on GitHub.

* Bump the version in anticipation of the next release, e.g. [after 8.13.3 release](https://github.com/elastic/apm-server/pull/13066) bump to 8.13.4. **Prepare this PR ahead of time** but only merge it once pinged by the Release Manager on release date.

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
