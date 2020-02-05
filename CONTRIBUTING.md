# Contributing to the APM Server

The APM Server is open source and we love to receive contributions from our community — you!

There are many ways to contribute,
from writing tutorials or blog posts,
improving the documentation,
submitting bug reports and feature requests or writing code.

You can get in touch with us through [Discuss](https://discuss.elastic.co/c/apm),
feedback and ideas are always welcome.

## Code contributions

If you have a bugfix or new feature that you would like to contribute,
please find or open an issue about it first.
Talk about what you would like to do.
It may be that somebody is already working on it,
or that there are particular issues that you should know about before implementing the change.

You will have to fork the `apm-server` repo,
please follow the instructions in the [readme](README.md).

### Workflow

All feature development and most bug fixes hit the master branch first.
Pull requests should be reviewed by someone with commit access.
Once approved,
the author of the pull request,
or reviewer if the author does not have commit access,
should "Squash and merge".

### Backports

Before or during review,
a committer will tag the pull request with the target version(s).
Once a version is released,
new features are frozen for that minor version and will not be backported.
For example,
if 6.2 was just released,
the soonest a new feature will be released is 6.3,
not 6.2.1.
Breaking changes may need to wait until the next major version.
See [semver](https://semver.org/) for general information about major/minor versions.
Bug fixes may be backported on a case by case basis.
The committer of the original pull request,
typically the author,
is responsible for backporting the changes to the target versions.
Each backport is performed through its own pull request,
tagged with the target version and "backport",
and merged with "Create a merge commit".
Straightforward backports may be merged without review.

[Backport](https://github.com/sqren/backport) is recommended for automating the backport process.
