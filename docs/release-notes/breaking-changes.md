---
navigation_title: "Breaking changes"
---

# Elastic APM breaking changes [elastic-apm-breaking-changes]
Breaking changes can impact your Elastic applications, potentially disrupting normal operations. Before you upgrade, carefully review the Elastic APM breaking changes and take the necessary steps to mitigate any issues. To learn how to upgrade, check out [Upgrade](docs-content://solutions/observability/apps/upgrade.md).

% ## Next version

% **Release date:** Month day, year

% ::::{dropdown} Title of breaking change
% Description of the breaking change.
% For more information, check [PR #](PR link).
% **Impact**<br> Impact of the breaking change.
% **Action**<br> Steps for mitigating deprecation impact.
% ::::

## 9.0.0 [elastic-apm-900-breaking-changes]
**Release date:** April 2, 2025

::::{dropdown} Change server information endpoint "/" to only accept GET and HEAD requests
This will surface any agent misconfiguration causing data to be sent to `/` instead of the correct endpoint (for example, `/v1/traces` for OTLP/HTTP).
For more information, check [PR #15976](https://github.com/elastic/apm-server/pull/15976).

**Impact**<br>Any methods other than `GET` and `HEAD` to `/` will return HTTP 405 Method Not Allowed.

**Action**<br>Update any existing usage, for example, update `POST /` to `GET /`.
::::

::::{dropdown} The Elasticsearch apm_user role has been removed
The Elasticsearch `apm_user` role has been removed.
For more information, check [PR #14876](https://github.com/elastic/apm-server/pull/14876).

**Impact**<br>If you are relying on the `apm_user` to provide access, users may lose access when upgrading to the next version.

**Action**<br>After this change if you are relying on `apm_user` you will need to specify its permissions manually.
::::

::::{dropdown} The sampling.tail.storage_limit default value changed to 0
The `sampling.tail.storage_limit` default value changed to `0`. While `0` means unlimited local tail-sampling database size, it now enforces a max 80% disk usage on the disk where the data directory is located. Any tail sampling writes that occur after this threshold will be rejected, similar to what happens when tail-sampling database size exceeds a non-0 storage limit. Setting `sampling.tail.storage_limit` to non-0 maintains the existing behavior, which limits the tail-sampling database size to `sampling.tail.storage_limit` and does not have the new disk usage threshold check.
For more information, check [PR #15467](https://github.com/elastic/apm-server/pull/15467) and [PR #15524](https://github.com/elastic/apm-server/pull/15524).

**Impact**<br>If `sampling.tail.storage_limit` is already set to a non-`0` value, tail sampling will maintain the existing behavior.
If you're using the new default, it will automatically scale with a larger disk.

**Action**<br>To continue using the existing behavior, set the `sampling.tail.storage_limit` to a non-`0` value.
To use the new disk usage threshold check, set the `sampling.tail.storage_limit` to `0` (the default value).
::::

::::{dropdown} Tail-based sampling database files from 8.x are ignored
Due to a change in underlying tail-based sampling database, the 8.x database files are ignored when running APM Server 9.0+.

**Impact**<br>There is a limited, temporary impact around the time when an upgrade happens. Any locally stored events awaiting tail sampling decision before upgrading to 9.0 will effectively be ignored, as if their traces are unsampled. If there are many APM Server making tail sampling decisions, it may result in broken traces.

**Action**<br>No action needed. There is no impact on traces ingested after upgrade.
::::

::::{dropdown} Old tail-based sampling database files from 8.x consume unnecessary disk space
As tail-based sampling database files from version 8.x are now ignored and consume unnecessary disk space, some files can be deleted to reclaim disk space.
It does not affect Elastic Cloud Hosted, as storage is automatically cleaned up by design.

**Impact**<br>Unnecessary disk usage, except for Elastic Cloud Hosted users.

**Action**<br>To clean up the unnecessary files, first identify APM Server data path, configured via `path.data`, which is also usually printed with "Data path: " in APM Server logs on startup. Assuming the environment variable `PATH_DATA` is the data path for TBS we identified above, the files that should be deleted are `KEYREGISTRY`, `LOCK`, `MANIFEST`, `*.vlog`, `*.sst` under `$PATH_DATA/tail_sampling/`. Do not delete other files.
::::
