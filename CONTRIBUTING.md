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

### Submitting your changes

Generally,
we require that you test any code you are adding or modifying.
Once your changes are ready to submit for review:

1. Sign the Contributor License Agreement

    Please make sure you have signed our [Contributor License Agreement](https://www.elastic.co/contributor-agreement/).
    We are not asking you to assign copyright to us,
    but to give us the right to distribute your code without restriction.
    We ask this of all contributors in order to assure our users of the origin and continuing existence of the code.
    You only need to sign the CLA once.

2. Test your changes

    Run the test suite to make sure that nothing is broken.
    See [testing](TESTING.md) for details.

3. Rebase your changes

    Update your local repository with the most recent code from the main repo,
    and rebase your branch on top of the latest master branch.
    We prefer your initial changes to be squashed into a single commit.
    Later,
    if we ask you to make changes,
    add them as separate commits.
    This makes them easier to review.
    As a final step before merging we will either ask you to squash all commits yourself or we'll do it for you.

4. Submit a pull request

    Push your local changes to your forked copy of the repository and [submit a pull request](https://help.github.com/articles/using-pull-requests).
    In the pull request,
    choose a title which sums up the changes that you have made,
    and in the body provide more details about what your changes do.
    Also mention the number of the issue where discussion has taken place,
    eg "Closes #123".

5. Be patient

    We might not be able to review your code as fast as we would like to,
    but we'll do our best to dedicate it the attention it deserves.
    Your effort is much appreciated!

### Workflow

All feature development and most bug fixes hit the main branch first.
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

### Examples

This is a collection of example PRs for additions occuring somewhat frequently.

* [Adding a new field to the Intake API and index it in Elasticsearch](https://github.com/elastic/apm-server/pull/4626#issue-555484976)
