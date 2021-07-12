# APM Server release checklist

## Before the release

* Backport all relevant commits

  e.g. start with following command to see which files differ that are not necessarily expected:
  `git diff elastic/master --name-only | grep -v '^vendor/' | grep -v '^docs/' | grep -v '^_beats'`

* Update beats

  run `make update-beats` in the branch from which the new branch will be created before FF to recognize potential issues

* Update Kibana Index Pattern

  If fields are not up to date, perform the following and open a PR:
  * Fork and locally clone kibana
  * Ensure that your copy of kibana has the current release branch from elastic/kibana
  * From the apm-server repo run:
    `make update && script/update_kibana_objects.py -d path/to/your/kibana -b $release_branch`

* Update Changelog

  * Review existing [changelogs/head](https://github.com/elastic/apm-server/tree/master/changelogs/head.asciidoc) to ensure all relevant notes have been added
  * Move changelogs from _head_ to _release_version_:
    * Minor version: Create new changelog file from [changelogs/head.asciidoc](https://github.com/elastic/apm-server/blob/master/changelogs/head.asciidoc)
      If changes should not be backported, keep them in the _changelogs/head.asciidoc_ file.
    * Patch version: Add new section to existing release notes. ([Sample PR](https://github.com/elastic/apm-server/pull/2064/files))

    Create PR in `master` and backport.

  * Run the [`check_changelogs.py`](script/check_changelogs.py) script to ensure changelogs are synced across branches. This will soon be a PR check.
  * Don't forget to update the "SUPPORTED_VERSIONS" to include a new branch if necessary.

* For minor releases create a new release branch and
  * update versions in release branch, e.g. [#2803](https://github.com/elastic/apm-server/pull/2803/files)
  * update versions in `major.x` branch to next minor version, e.g. [#2804](https://github.com/elastic/apm-server/pull/2804)

* Update to latest changes of [beats](https://github.com/elastic/beats/pulls/)
  * Update `BEATS_VERSION` to the release version in the top-level Makefile
  * When beats has merged all PRs and for minor releases created the new branch, run `make update-beats` and commit the changes.

* Ensure a branch or tag is created for the [go-elasticsearch](https://github.com/elastic/go-elasticsearch) library and update to it.

  `go get github.com/elastic/go-elasticsearch/v$major@$major.$minor`

* The following may also need to be updated manually:
    * APM Overview's [release highlights](https://github.com/elastic/apm-server/blob/master/docs/guide/apm-release-notes.asciidoc) - Anything exciting across the APM stack!
    * APM Overview's [breaking changes](https://github.com/elastic/apm-server/blob/master/docs/guide/apm-breaking-changes.asciidoc) - Any breaking changes across the APM stack.
    * APM Server's [breaking changes](https://github.com/elastic/apm-server/blob/master/docs/breaking-changes.asciidoc) - Any APM Server breaking changes.
    * APM Server's [upgrade guide](https://github.com/elastic/apm-server/blob/master/docs/upgrading.asciidoc).

* For major releases, update and smoke test the dev quick start [`docker-compose.yml`](https://github.com/elastic/apm-server/blob/master/docs/guide/docker-compose.yml).

## After feature freeze

* Update [.mergify.yml](https://github.com/elastic/apm-server/blob/master/.mergify.yml) with a new backport rule for the next version.

## On release day

* New branches need to be added to `conf.yml` in the `elastic/docs` repo. [Example](https://github.com/elastic/docs/pull/893/files#diff-4a701a5adb4359c6abf9b8e1cb38819fR925). **This is handled by the docs release manager.**

* Merge the above PRs

* Verify that a new [tag](https://github.com/elastic/apm-server/releases) has been created on GitHub.

* Bump the version in anticipation of the next release, e.g. [after 7.5.1 release](https://github.com/elastic/apm-server/pull/3045/files) bump to 7.5.2

  Prepare this PR ahead of time, but only merge after release!

## When compatibility between Agents & Server changes

* Update the [agent/server compatibility matrix](https://github.com/elastic/apm-server/blob/master/docs/guide/agent-server-compatibility.asciidoc).

## Templates

Templates for adding release notes, breaking changes, and highlights.

<details><summary><code>/changelogs/*.asciidoc</code> template</summary>

```asciidoc
[[release-notes-7.1]]
== APM Server version 7.1

https://github.com/elastic/apm-server/compare/7.0\...7.1[View commits]

* <<release-notes-7.1.0>>

[[release-notes-7.1.0]]
=== APM Server version 7.1.0

https://github.com/elastic/apm-server/compare/v7.0.1\...v7.1.0[View commits]

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
* <<release-highlights-7.1.0>>

[[release-highlights-7.1.0]]
=== APM version 7.1.0

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
* <<breaking-7.0.0, APM version 7.0.0>>

[[breaking-7.0.0]]
=== Breaking changes in 7.0.0

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
