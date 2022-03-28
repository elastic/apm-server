# Contributing to the APM Server

APM Server is open source, and we love to receive contributions from our community — you!

There are many ways to contribute,
from writing tutorials or blog posts,
improving the documentation,
submitting bug reports and feature requests, or writing code.

If you want to be rewarded for your contributions, sign up for the
[Elastic Contributor Program](https://www.elastic.co/community/contributor).
Each time you make a valid contribution, you’ll earn points that increase your chances of winning prizes and being recognized as a top contributor.

## Questions

GitHub is reserved for bug reports and feature requests; it is not the place
for general questions. If you have a question or an unconfirmed bug, please
visit our [discussion forum](https://discuss.elastic.co/c/apm);
feedback and ideas are always welcome.

## Code contributions

If you have a bug fix or new feature that you would like to contribute,
please find or open an issue first.
It's important to talk about what you would like to do,
as there may already be someone working on it,
or there may be context to be aware of before implementing the change.

Development instructions are available in the project [readme](README.md#apm-server-development).

### Submitting your changes

Please read our [pull request template](.github/pull_request_template.md), which includes the information we care about the most when submitting new changes.

### Workflow

All feature development and most bug fixes hit the main branch first.
Pull requests should be reviewed by someone with commit access.
Once approved,
the author of the pull request,
or reviewer if the author does not have commit access,
should "Squash and merge".

### Backports

Before or during review, a committer will label the pull request with `backport-<branch>` for the
prior release branches to which the changes should be backported. For example, say the next minor
release is 8.2, and a bug fix is implemented which should be backported to 8.1.next. The pull request
should be developed against main, with the `backport-8.1` label applied. Once the pull request is merged,
it will be automatically backported to the 8.1 branch.

Once a version is released, new features are frozen for that minor version and will not be backported.
For example, if 7.10 was just released, the soonest a new feature will be released is 7.11, not 7.10.1.

Breaking changes may need to wait until the next major version.
See [semver](https://semver.org/) for general information about major/minor versions.

Bug fixes may be backported on a case by case basis. The committer of the original pull request,
typically the author, is responsible for backporting the changes to the target versions.
Straightforward backports may be merged without review.

As an alternative to backport automation through `backport-<branch>` labels (e.g. when automation fails),
we sometimes use [Backport](https://github.com/sqren/backport).

### Examples

This is a collection of example PRs for additions occuring somewhat frequently.

* [Adding a new field to the Intake API and index it in Elasticsearch](https://github.com/elastic/apm-server/pull/4626#issue-555484976)
