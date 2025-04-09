# APM Server release checklist

The APM Server follows the Elastic Stack release schedule and versions.
A release starts with a Feature Freeze period, during which only bug fixes
are allowed to be merged into the specific release branch.
We generally follow [semver](https://semver.org/) for release versions.
For major and minor releases a new branch is cut off the main branch.
For patch releases, only the version on the existing major and minor version branch is updated.

Release documentation is split based on the major version the release is for:
- [8.x release guide](./RELEASE_8x.md)
- [9.x release guide](./RELEASE_9x.md)
