---
navigation_title: "Breaking changes"
mapped_pages:
  - https://www.elastic.co/guide/en/observability/current/apm-breaking.html
applies_to:
  stack: ga
products:
  - id: observability
  - id: apm
---

# Elastic APM breaking changes

Breaking changes can impact your Elastic applications, potentially disrupting normal operations. Before you upgrade, carefully review the Elastic APM breaking changes and take the necessary steps to mitigate any issues. To learn how to upgrade, check [Upgrade](docs-content://deploy-manage/upgrade.md).

## Next version [next-version]

% ::::{dropdown} Title of breaking change
% Description of the breaking change.
% For more information, check [#PR-number]({{apm-pull}}PR-number).
%
% **Impact**<br> Impact of the breaking change.
%
% **Action**<br> Steps for mitigating deprecation impact.
% ::::

## 9.0.0 [9-0-0]

::::{dropdown} Change server information endpoint "/" to only accept GET and HEAD requests
This will surface any agent misconfiguration causing data to be sent to `/` instead of the correct endpoint (for example, `/v1/traces` for OTLP/HTTP).
For more information, check [#15976]({{apm-pull}}15976).

**Impact**<br> Any methods other than `GET` and `HEAD` to `/` will return HTTP 405 Method Not Allowed.

**Action**<br> Update any existing usage, for example, update `POST /` to `GET /`.
::::

::::{dropdown} The Elasticsearch apm_user role has been removed
The Elasticsearch `apm_user` role has been removed.
For more information, check [#14876]({{apm-pull}}14876).

**Impact**<br>If you are relying on the `apm_user` to provide access, users may lose access when upgrading to the next version.

**Action**<br>After this change if you are relying on `apm_user` you will need to specify its permissions manually.
::::

::::{dropdown} The sampling.tail.storage_limit default value changed to 0
The `sampling.tail.storage_limit` default value changed to `0`. While `0` means unlimited local tail-sampling database size, it now enforces a max 80% disk usage on the disk where the data directory is located. Any tail sampling writes that occur after this threshold will be rejected, similar to what happens when tail-sampling database size exceeds a non-0 storage limit. Setting `sampling.tail.storage_limit` to non-0 maintains the existing behavior, which limits the tail-sampling database size to `sampling.tail.storage_limit` and does not have the new disk usage threshold check.
For more information, check [#15467]({{apm-pull}}15467) and [#15524]({{apm-pull}}15524).

**Impact**<br>If `sampling.tail.storage_limit` is already set to a non-`0` value, tail sampling will maintain the existing behavior.
If you're using the new default, it will automatically scale with a larger disk.

**Action**<br>To continue using the existing behavior, set the `sampling.tail.storage_limit` to a non-`0` value.
To use the new disk usage threshold check, set the `sampling.tail.storage_limit` to `0` (the default value).
::::

::::{dropdown} Tail-based sampling database files from 8.x are ignored
Due to a change in the underlying tail-based sampling database, the 8.x database files are ignored when running APM Server 9.0+.

**Impact**<br>There is a limited, temporary impact around the time when an upgrade happens. Any locally stored events awaiting tail sampling decision before upgrading to 9.0 will effectively be ignored, as if their traces are unsampled. If there are many APM Server making tail sampling decisions, it may result in broken traces.

**Action**<br>No action needed. There is no impact on traces ingested after upgrade.
::::

::::{dropdown} Old tail-based sampling database files from 8.x consume unnecessary disk space
As tail-based sampling database files from version 8.x are now ignored and consume unnecessary disk space, some files can be deleted to reclaim disk space.
It does not affect Elastic Cloud Hosted, as storage is automatically cleaned up by design.

**Impact**<br>Unnecessary disk usage, except for Elastic Cloud Hosted users.

**Action**<br>To clean up the unnecessary files, first identify APM Server data path, configured via `path.data`, which is also usually printed with "Data path: " in APM Server logs on startup. Assuming the environment variable `PATH_DATA` is the data path for TBS we identified above, the files that should be deleted are `KEYREGISTRY`, `LOCK`, `MANIFEST`, `*.vlog`, `*.sst` under `$PATH_DATA/tail_sampling/`. Do not delete other files.
::::

::::{dropdown} Removal of Fleet AgentCfg fetcher
APM Server no longer supports the agent configuration fetching mechanism where Fleet injects agent configuration into
the APM Server
at regular intervals.
The default behavior is now to fetch agent configuration directly from Elasticsearch.
Fetching the agent configuration from Kibana is still supported when APM Server is configured without an Elasticsearch
output.

For more information, check [#14539]({{apm-pull}}14539).
::::

::::{dropdown} Removal of the `apiKey` subcommand
The `apiKey` subcommand has been removed from the `apm-server` binary.

For more information, check [#14127]({{apm-pull}}14127).

**Impact**<br> Creating API keys for ingestion is not supported through the CLI.

**Action**<br> Use Kibana to create API keys for ingestion.
::::

::::{dropdown} Remove `TLSv1.1` from default configuration
TLS version 1.1 protocol is not included anymore in the list of supported protocols in the default `apm-server.yml`
configuration file.
TLS 1.1 is considered insecure and is not recommended for use.
The `ssl.supported_protocols` setting now defaults to `[TLSv1.2, TLSv1.3]`.

For more information, check [#14790]({{apm-pull}}14790).

**Impact**<br> The default configuration doesn't allow TLSv1.1 connections.

**Action**<br> Modify the `apm-server.yml` or APM integration policy `ssl.supported_protocols` setting to include `TLSv1.1` if needed.
::::

::::{dropdown} Removal of `TLSv1.0` support
TLS version 1.0 protocol is no longer supported.

For more information, check [#10491]({{apm-issue}}10491).

**Impact**<br> Existing TLS version configuration that contains `TLSv1.0` will cause APM Server to fail to start.

**Action**<br> Modify the `apm-server.yml` or APM integration policy `ssl.supported_protocols` setting to remove `TLSv1.0`.
::::

::::{dropdown} Removal of deprecated RUM configuration options
the following deprecated options in the RUM configuration have been removed:

- `event_rate.limit`
- `event_rate.lru_size`
- `allow_service_names`

**Impact**<br> The aforementioned configurations are ignored.

**Action**<br> Refer to the RUM section in APM documentation to use non-deprecated options.
::::

::::{dropdown} Change internal error grouping key
The `error.grouping_key` field in events processed by APM Server is now calculated using `xxhash` instead of `md5`.
Thus, the format of the `error.grouping_key` field has changed, making it different with documents ingested in previous
versions.

For more information, check [#15211]({{apm-pull}}15211).

**Impact**<br> The `error.grouping_key` field has a different format than previous versions.
::::

::::{dropdown} Remove support for Jaeger API
Jaeger API support was deprecated in 8.16. APM Server no longer supports the Jaeger API for ingesting distributed
traces.

For more information, check [#14791]({{apm-pull}}14791).

**Impact**<br> Events cannot be ingested anymore with the Jaeger gRPC API.

**Action**<br> Migrate to APM or OTLP.
::::
