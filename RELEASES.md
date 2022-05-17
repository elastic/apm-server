# APM Server release checklist

The APM Server follows the Elastic Stack release schedule and versions.
A release starts with a Feature Freeze period, during which only bug fixes
are allowed to be merged into the specific release branch.
We generally follow [semver](https://semver.org/) for release versions.
For major and minor releases a new branch is cut off the main branch.
For patch releases, only the version on the existing major and minor version branch is updated.

## Before Feature Freeze

* Backport all relevant commits

  We use backport labels on PRs for automated backporting of changes.
  Everything relevant to a release should already be backported.
  Nevertheless, a quick check to ensure nothing has been overlooked, should be done.
  For example, use following command to see which files differ:
  `git diff elastic/main --name-only | grep -v '^vendor/' | grep -v '^docs/' | grep -v '^_beats'`

* Update beats

  PRs for updating libbeat dependencies are automatically created multiple times per week.
  Ensure that these updates have been merged for the release branch.
  If there is need for a manual update, you can run `make update-beats` on the release branch.

* Update Changelog

  * Review existing [changelogs/head](https://github.com/elastic/apm-server/tree/main/changelogs/head.asciidoc) to ensure all relevant notes have been added
  * Move changelogs from _head_ to _release_version_:
    * Minor version: Create new changelog file from [changelogs/head.asciidoc](https://github.com/elastic/apm-server/blob/main/changelogs/head.asciidoc)
      If changes should not be backported, keep them in the _changelogs/head.asciidoc_ file.
    * Patch version: Add new section to existing release notes. ([Sample PR](https://github.com/elastic/apm-server/pull/2064/files))

    Create the changelog PR in `main` and backport to the release branch.

  * The [`check_changelogs.py`](script/check_changelogs.py) script is run as a PR check, ensuring that changelog changes are synced across branches.
  * Don't forget to update the "SUPPORTED_VERSIONS" to include a new branch if necessary.

* For minor releases create a new release branch and
  * update versions in release branch, e.g. [#2803](https://github.com/elastic/apm-server/pull/2803/files)
  * update versions in `major.x` branch to next minor version, e.g. [#2804](https://github.com/elastic/apm-server/pull/2804)

* Ensure a branch or tag is created for the [go-elasticsearch](https://github.com/elastic/go-elasticsearch) library and update to it.

  `go get github.com/elastic/go-elasticsearch/v$major@$major.$minor`

* The following may also need to be updated manually:
    * APM Overview's [release highlights](https://github.com/elastic/apm-server/blob/main/docs/guide/apm-release-notes.asciidoc) - Anything exciting across the APM stack!
    * APM Overview's [breaking changes](https://github.com/elastic/apm-server/blob/main/docs/guide/apm-breaking-changes.asciidoc) - Any breaking changes across the APM stack.
    * APM Server's [breaking changes](https://github.com/elastic/apm-server/blob/main/docs/breaking-changes.asciidoc) - Any APM Server breaking changes.
    * APM Server's [upgrade guide](https://github.com/elastic/apm-server/blob/main/docs/upgrading.asciidoc).

* For major releases, update and smoke test the dev quick start [`docker-compose.yml`](https://github.com/elastic/apm-server/blob/main/docs/guide/docker-compose.yml).

## After feature freeze

* Update [.mergify.yml](https://github.com/elastic/apm-server/blob/main/.mergify.yml) with a new backport rule for the next version.

* Update libbeat dependencies

  When beats has merged all PRs and for minor releases created the new branch, update the `BEATS_VERSION` to the release version in the top-level Makefile.
  Beats updates should be automatically created for the release branch. Keep an eye out for the changes they introduce.
  The updates are supposed to only be bug fixes, in which cases the updates should be merged in during Feature Freeze period.

* Create a test plan

  Create a github issue for testing the release branch. 
  It should contain 
  * a link to all PRs in the APM Server repository that need to be tested manually. Use the test-plan* labels and the version labels 
    to create an overview over the PRs that need testing. For example, [test plan link for 8.2.0](https://github.com/elastic/apm-server/issues?q=label%3Atest-plan+is%3Aclosed+label%3Av8.2.0).
  * add test scenarios outside the APM Server repository that require manual testing, such as test scenarios on ESS or 
    OS compatibility smoke tests if support for new operating systems is introduced.  

## On release day

* New branches need to be added to `conf.yml` in the `elastic/docs` repo. [Example](https://github.com/elastic/docs/pull/893/files#diff-4a701a5adb4359c6abf9b8e1cb38819fR925). **This is handled by the docs release manager.**

* Verify that a new [tag](https://github.com/elastic/apm-server/releases) has been created on GitHub.

* Bump the version in anticipation of the next release, e.g. [after 7.5.1 release](https://github.com/elastic/apm-server/pull/3045/files) bump to 7.5.2

  Prepare this PR ahead of time, but only merge after release!

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
