# APM Server release checklist

## Before the release

* Create a PR that:
    1. Uncomments out relevant release notes. Release notes are in the [changelogs](https://github.com/elastic/apm-server/tree/master/changelogs).
    2. Updates the `:stack-version:`
    * [Sample PR](https://github.com/elastic/apm-server/pull/2064/files)

* Update `vendor/github.com/elastic/beats/libbeat/version/version.go`
    * [Sample PR](https://github.com/elastic/apm-server/pull/1886)

* The following may also need to be updated manually:
    * APM Overview's [release highlights](https://github.com/elastic/apm-server/blob/master/docs/guide/apm-release-notes.asciidoc) - Anything exciting across the APM stack!
    * APM Overview's [breaking changes](https://github.com/elastic/apm-server/blob/master/docs/guide/apm-breaking-changes.asciidoc) - Any breaking changes across the APM stack.
    * APM Server's [breaking changes](https://github.com/elastic/apm-server/blob/master/docs/breaking-changes.asciidoc) - Any APM Server breaking changes.
    * APM Server's [upgrade guide](https://github.com/elastic/apm-server/blob/master/docs/upgrading.asciidoc).

* Script for determining if changelogs are synced: https://gist.github.com/graphaelli/92fd6b5ff08d69600e880df2a234ac35
    * This will soon be a PR check

## During the release

* New branches need to be added to `conf.yml` in the `elastic/docs` repo. [Example](https://github.com/elastic/docs/pull/893/files#diff-4a701a5adb4359c6abf9b8e1cb38819fR925). **This is handled by the docs release manager.**

* Merge the above PRs

## When compatibility between Agents & Server changes

1. Update the [agent/server compatibility matrix](https://github.com/elastic/apm-server/blob/master/docs/guide/agent-server-compatibility.asciidoc).

2. Update the version in APM Server's [`/docs/version.asciidoc`](https://github.com/elastic/apm-server/blob/master/docs/version.asciidoc). This ensures cross document links point to the correct documentation version.

3. Ensure the agent points to the correct server version by changing the `:branch:` attribute.
    * Example: [Node 1.x](https://raw.githubusercontent.com/elastic/apm-agent-nodejs/1.x/docs/index.asciidoc) vs. [Node Master](https://raw.githubusercontent.com/elastic/apm-agent-nodejs/master/docs/index.asciidoc)

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
