# APM Server release checklist

## Before the release

* Backport all relevant commits

* Create a PR that uncomments out relevant release notes. Release notes are in the [changelogs](https://github.com/elastic/apm-server/tree/master/changelogs).
    * [Sample PR](https://github.com/elastic/apm-server/pull/2064/files)

* Create a separate PR to update `vendor/github.com/elastic/beats/libbeat/version/version.go`
    * [Sample PR](https://github.com/elastic/apm-server/pull/1886)

* The following may also need to be updated manually:
    * APM Overview's [release highlights](https://github.com/elastic/apm-server/blob/master/docs/guide/apm-release-notes.asciidoc) - Anything exciting across the APM stack!
    * APM Overview's [breaking changes](https://github.com/elastic/apm-server/blob/master/docs/guide/apm-breaking-changes.asciidoc) - Any breaking changes across the APM stack.
    * APM Server's [breaking changes](https://github.com/elastic/apm-server/blob/master/docs/breaking-changes.asciidoc) - Any APM Server breaking changes.
    * APM Server's [upgrade guide](https://github.com/elastic/apm-server/blob/master/docs/upgrading.asciidoc).

* Changelogs:
    * Review the [changelogs](https://github.com/elastic/apm-server/tree/master/changelogs) to ensure all relevant notes have been added
    * Run the [`check_changelogs.py`](script/check_changelogs.py) script to ensure changelogs are synced across branches. This will soon be a PR check.
        * Don't forget to update the "VERSIONS" to include a new branch if necessary.

* For major releases, update and smoke test the dev quick start [`docker-compose.yml`](https://github.com/elastic/apm-server/blob/master/docs/guide/docker-compose.yml).

## On release day

* New branches need to be added to `conf.yml` in the `elastic/docs` repo. [Example](https://github.com/elastic/docs/pull/893/files#diff-4a701a5adb4359c6abf9b8e1cb38819fR925). **This is handled by the docs release manager.**

* Merge the above PRs

* Verify that a new [tag](https://github.com/elastic/apm-server/releases) has been created on GitHub.

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
==== Added

[float]
==== Removed

[float]
==== Bug fixes
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
