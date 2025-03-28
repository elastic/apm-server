# Smoke Tests

This directory contains the smoke tests that are run by CI (`smoke-test`). 

There are 2 main categories of smoke tests that we run:
  - Upgrade tests
  - OS support tests

## Upgrade Tests

Upgrade tests (`smoke-test-ess`) are tests that checks if APM Server works properly after upgrades to the stack on Elastic Cloud Hosted. There are 2 types of upgrades:
  - Version upgrades
  - Standalone to managed upgrades

### Version Upgrades

Version upgrade tests checks if APM Server works properly after the stack has been upgraded from one version to another.
The tests reside in `basic-upgrade/`.

The APM Server versions that are tested is defined in `smoke-test-ess.yml` under `matrix.version`.
The test first finds the previous latest version, deploys APM stack in that version, then upgrades the stack to the tested version.
Finally, it checks that APM Server is ingesting events as expected.

Note: The version upgrades can either be patch, minor or major upgrades. For example:
  - Patch: `7.17.27` -> `7.17.28`
  - Minor: `8.17.3`  -> `8.18.0`
  - Major: `8.18.0`  -> `9.0.0`

### Standalone to Managed Upgrades

Standalone to managed upgrade tests mainly checks if APM Server works properly after APM switches from standalone to managed.
Standalone here refers to the APM Server binary being deployed by itself, whereas managed refers to APM Server being Fleet-managed, see [here](https://www.elastic.co/guide/en/observability/current/apm-getting-started-apm-server.html) to find out more.
The tests reside in `legacy-managed/` and `legacy-standalone-major-managed/`.

Currently, we only run these tests starting from `7.x` versions, mainly because Elastic Cloud disallows creation of standalone APM Server since `8.0`.
The tests that we have are essentially:
  - `7.17.x` standalone -> `7.17.x+1` standalone -> `7.17.x+1` managed
  - `7.17.x` standalone -> latest `8.x` standalone -> latest `9.x` standalone -> latest `9.x` managed

## OS Support Tests

OS support tests (`smoke-test-os`) are tests that checks if upcoming APM Server versions work properly in operating systems that [Elastic pledged to support](https://www.elastic.co/support/matrix#matrix_os).
The tests reside in `supported-os/`, and the OS support matrix can be found in `os_matrix.sh`.

The APM Server versions that are tested comes from [active-branches](https://github.com/elastic/oblt-actions/tree/main/elastic/active-branches).
For each version, the test will deploy standalone APM Server to all supported OS on AWS EC2, and check that APM Server ingests events as expected.
