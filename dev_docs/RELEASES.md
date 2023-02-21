# APM Server release checklist

The APM Server follows the Elastic Stack release schedule and versions.
A release starts with a Feature Freeze period, during which only bug fixes
are allowed to be merged into the specific release branch.
We generally follow [semver](https://semver.org/) for release versions.
For major and minor releases a new branch is cut off the main branch.
For patch releases, only the version on the existing major and minor version branch is updated.

## Feature Freeze

* For patch releases, ensure all relevant backport PRs are merged. 
  We use backport labels on PRs and automation to ensure labels are set.

* Update Changelog

  * Review existing [changelogs/head](https://github.com/elastic/apm-server/tree/main/changelogs/head.asciidoc) to ensure all relevant notes have been added.
  * Move changelog entries from _head_ to _release_version_:
    * Minor version:
      Create new changelog file from [changelogs/head.asciidoc](https://github.com/elastic/apm-server/blob/main/changelogs/head.asciidoc)
      If changes should not be backported, keep them in the _changelogs/head.asciidoc_ file.
      Don't forget to `include` and link to the new file in [main changelog](https://github.com/elastic/apm-server/blob/main/CHANGELOG.asciidoc) and the [release notes](https://github.com/elastic/apm-server/blob/main/docs/release-notes.asciidoc) file. [(Sample PR)](https://github.com/elastic/apm-server/pull/7956/files)
    * Patch version: Add a new section to existing release notes. ([Sample PR](https://github.com/elastic/apm-server/pull/8313/files))
  * Add `@elastic/obs-docs` as a reviewer.

## Day after Feature Freeze

* For minor releases, cut a new release branch from `main` and update them.
  * Release branch:
    Update versions and ensure that the `BEATS_VERSION` in the Makefile is updated,
    e.g. [#2803](https://github.com/elastic/apm-server/pull/2803/files).
    Trigger a new beats update, once the beats branch is also created.
    Remove the [changelogs/head.asciidoc](https://github.com/elastic/apm-server/blob/main/changelogs/head.asciidoc) file from the release branch. 

  * Main branch: 
    Update [.mergify.yml](https://github.com/elastic/apm-server/blob/main/.mergify.yml) with a new backport rule for the next version,
    and update versions to next minor version, e.g. [#2804](https://github.com/elastic/apm-server/pull/2804).

  The release manager will ping the teams, but you can already prepare this in advance on the day after Feature Freeze.

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

  Create a github issue for testing the release branch (follow the GitHub issue template for the test plan), It should contain:
  * A link to all PRs in the APM Server repository that need to be tested manually. Use the `test-plan*` labels and the version labels
    to create an overview over the PRs that need testing. For example, [test plan link for 8.3.0](https://github.com/elastic/apm-server/issues?q=label%3Atest-plan+is%3Aclosed+label%3Av8.3.0).
  * Add other test cases that require manual testing, such as test scenarios on ESS, that are not covered by automated tests or
    OS compatibility smoke tests for supporting new operating systems.

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

* Bump the version in anticipation of the next release, e.g. [after 7.5.1 release](https://github.com/elastic/apm-server/pull/3045/files) bump to 7.5.2. Prepare this PR ahead of time, but only merge after release when pinged by the release manager.

* Ensure that the `apmpackage` is released to production (supposed to change in `8.5`).

## When compatibility between Agents & Server changes

* Update the [agent/server compatibility matrix](https://github.com/elastic/apm-server/blob/main/docs/guide/agent-server-compatibility.asciidoc).

## Templates

Templates for adding release notes, breaking changes, and highlights.

<details><summary><code>/changelogs/*.asciidoc</code> template</summary>

```asciidoc
[[release-notes-8.1]]
== APM Server version 8.1

https://github.com/elastic/apm-server/compare/8.0\...8.1[View commits]

* <<release-notes-8.1.0>>

[[release-notes-8.1.0]]
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
