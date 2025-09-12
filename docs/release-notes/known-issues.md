---
mapped_pages:
  - https://www.elastic.co/guide/en/observability/current/apm-known-issues.html

navigation_title: "Known issues"
---

# Elastic APM known issues
Known issues are significant defects or limitations that may impact your implementation. These issues are actively being worked on and will be addressed in a future release. Review the Elastic APM known issues to help you make informed decisions, such as upgrading to a new version.

% Use the following template to add entries to this page.

% :::{dropdown} Title of known issue
% **Applicable versions for the known issue and the version for when the known issue was fixed**
% On [Month Day, Year], a known issue was discovered that [description of known issue].
% For more information, check [Issue #](Issue link).

% **Workaround**
% Workaround description.
% :::

:::{dropdown} APM occasionally returning HTTP 502 "backend connection closed" or "use of closed network connection"

*Elastic Stack versions: all until fixed versions*
*Environments: ECH, ECE

APM Server on ECH and ECE might sometimes return HTTP 502 with error message "backend connection closed" or "use of closed network connection" for any requests due to a rare race condition.
When this happens to an intake request, Elastic APM agents will log an error but will not retry, leading to data loss.

Note that there may be other causes to "backend connection closed" or "use of closed network connection", and the provided workaround and released bugfix will only resolve the case related to the mentioned race condition.

**Workaround**

To work around this issue:

- Go to **Kibana** > **Fleet** > **Elastic Cloud agent policy**,
- Next to **Elastic APM**, select the **...** icon, then **Edit Integration**.
- Under **General**, select **Advanced options**, then change **Idle time before underlying connection is closed** to **200s**.
- Select **Save Integration**

This bug will be fixed in 8.18.7, 8.19.4, 9.0.7, 9.1.4 for new deployments, and 8.18.8, 8.19.5, 9.0.8, 9.1.5, 9.2.0 for upgraded deployments.
:::

:::{dropdown} APM Integration might be unreachable after upgrading to 8.19.0 and 9.1.0

*Elastic Stack versions: 8.19.0 and 9.1.0*
*Environments: ECH, ECE, ECK, and self-managed when running Fleet*

APM Integration, which is APM Server managed by Fleet, might sometimes be unreachable after reloading due to an integration policy change. Standalone APM Server is not affected by this issue.

When this happens, APM and OTLP intake through APM Integration will stop working, and there will be little to no logs from it. On ECH and ECE cloud, "Copy endpoint" will be grayed out.

**Workaround**

To work around this issue you can either:

- Restart the Integration servers through Force Restart in the Cloud Admin UI.
- Save a copy of the Elastic APM Integration policy within the affected policy, for example the Elastic Cloud agent policy, in the Fleet UI:
  - Go to **Kibana** > **Fleet** > the affected policy (e.g. **Elastic Cloud agent policy**),
  - Next to **Elastic APM**, select the **...** icon, then **Edit Integration**.
  - Add a blank space to the **Description**, then remove it.
  - Select **Save Integration**

In both cases, the settings of APM Integration are maintained. However, these workarounds will only keep APM Integration healthy until next integration policy change.

This bug will be fixed in 8.19.1 and 9.1.1.
:::

:::{dropdown} Tail Sampling may not compact / expired TTLs as quickly as desired, causing increased storage usage.

*Elastic Stack versions: 8.0.0+ < 9.0*

There are some issues with the Tail Sampling implementation in versions 8.0.0+ < 9.0 that may cause the buffered traces to not be compacted or expired as quickly as desired. This can lead to increased storage usage for longer than the default 30m TTL.

This may manifest in two ways, increased value log (vlog) file size and increased SST (LSM) file size. LSM growth and late compaction is particularly troublesome given how the underlying K/V database performs compactions on its layers. There is noticeable LSM growth for use-cases where traces are under 1KB in size, since they are written to the LSM layer directly.

This issue is fixed in 9.0.0, due to a re-implementation of how the underlying tail sampling databases are used. The new implementation uses a more efficient partitioning scheme, allowing more efficient expiration of traces.

:::

:::{dropdown} prefer_ilm required in component templates to create custom lifecycle policies

*Elastic Stack versions: 8.15.1+*

The issue occurs when creating a *new* cluster using version 8.15.1+. The issue occurs for any APM data streams created in 8.15.1+. The issue does *not* occur if custom component template has been created in or before version 8.15.0.

In 8.15.0, APM Server began using the [apm-data plugin](https://github.com/elastic/elasticsearch/tree/main/x-pack/plugin/apm-data) to manage data streams, ingest pipelines, lifecycle policies, and more. In 8.15.1, a fix was introduced to address unmanaged indices in older clusters using default ILM policies. This fix added a fallback to the default ILM policy (if it exists) and set the `prefer_ilm` configuration to `false`. This setting impacts clusters where both ILM and data stream lifecycles (DSL) are in effect—such as when configuring custom ILM policies using `@custom` component templates, under the conditions mentioned above.

To override ILM policies for these new clusters using component template, set the `prefer_ilm` configuration to `true` by following the [updated guide to customize ILM](docs-content://solutions/observability/apm/index-lifecycle-management.md).

:::

:::{dropdown} Upgrading to v8.15.x may cause ingestion to fail

*Elastic Stack versions: 8.15.0, 8.15.1, 8.15.2, 8.15.3*<br> *Fixed in Elastic Stack version 8.15.4*

The issue only occurs when *upgrading* the {{stack}} from 8.12.2 or lower directly to any 8.15.x version prior to 8.15.4. The issue does *not* occur when creating a *new* cluster using any 8.15.x version, or when upgrading from 8.12.2 to 8.13.x or 8.14.x and then to 8.15.x.

In APM Servers versions prior to 8.13.0, an ingestion pipeline exists to perform a check on the version. The version check would fail any APM document produced with a different version of APM server compared to the version of the installed APM’s ingest pipeline. In 8.13.0 the version check in the ingest pipeline was removed. Due to the combination of an internal change in how apm data management assets are set up from 8.15 onwards and a bug in Elasticsearch, related to [lazy rollover of data streams](https://github.com/elastic/elasticsearch/issues/112781), the ingestion pipeline conducting the version check is not removed on upgrade and prevents the ingestion of data.

If the deployment is running 8.15.0, upgrade the deployment to 8.15.1 or above. A manual rollover of all APM data streams is required to pick up the new index templates and remove the faulty ingest pipeline version check. Perform the following requests to Elasticsearch (they are assuming the `default` namespace is used, adjust if necessary):

```txt
POST /traces-apm-default/_rollover
POST /traces-apm.rum-default/_rollover
POST /logs-apm.error-default/_rollover
POST /logs-apm.app-default/_rollover
POST /metrics-apm.app-default/_rollover
POST /metrics-apm.internal-default/_rollover
POST /metrics-apm.service_destination.1m-default/_rollover
POST /metrics-apm.service_destination.10m-default/_rollover
POST /metrics-apm.service_destination.60m-default/_rollover
POST /metrics-apm.service_summary.1m-default/_rollover
POST /metrics-apm.service_summary.10m-default/_rollover
POST /metrics-apm.service_summary.60m-default/_rollover
POST /metrics-apm.service_transaction.1m-default/_rollover
POST /metrics-apm.service_transaction.10m-default/_rollover
POST /metrics-apm.service_transaction.60m-default/_rollover
POST /metrics-apm.transaction.1m-default/_rollover
POST /metrics-apm.transaction.10m-default/_rollover
POST /metrics-apm.transaction.60m-default/_rollover
```

:::

:::{dropdown} Upgrading to v8.15.0 may cause APM indices to lose their lifecycle policy

*Elastic Stack versions: 8.15.0*<br> *Fixed in Elastic Stack version 8.15.1*

The issue only occurs when *upgrading* the {{stack}} to 8.15.0. The issue does *not* occur when creating a *new* cluster using 8.15.0. The issue also does not occur if a custom ILM policy is configured using a custom component template.

In 8.15.0, APM Server switched to use data stream lifecycle to manage data retention for APM indices for new deployments as well as for upgraded deployments with default lifecycle configurations. Unfortunately, since any data stream created before 8.15.0 does not have a data stream lifecycle configuration, such existing data streams become unmanaged for default lifecycle configurations.

Upgrading to 8.15.1 resolves the lifecycle issue for any new indices created for APM data streams. However, indices created in version 8.15.0 will remain unmanaged if the default ILM policy is in place. To fix these unmanaged indices, consider one of the following approaches:

1. Manually delete the unmanaged indices when they are no longer needed.
2. Explicitly configure APM data streams to use the default data stream lifecycle configuration. This approach migrates all affected data streams to use data stream lifecycles, maintaining behavior equivalent to the default ILM policies. Apply this fix only to data streams that have unmanaged indices due to missing default ILM policies.

    ```txt
    PUT _data_stream/{{data_stream_name}}-{{data_stream_namespace}}/_lifecycle
    {
      "data_retention": <data_retention_period>
    }
    ```


Default `<data_retention_period>` for each data stream is available in [this guide](docs-content://solutions/observability/apm/index-lifecycle-management.md).

This issue is fixed in 8.15.1 ([elastic/elasticsearch#112432](https://github.com/elastic/elasticsearch/pull/112432)).

:::

:::{dropdown} Upgrading to v8.13.0 to v8.13.2 breaks APM anomaly rules [broken-apm-anomaly-rule]
*Elastic Stack versions: 8.13.0, 8.13.1, 8.13.2*<br> *Fixed in Elastic Stack version 8.13.3*

This issue occurs when upgrading the Elastic Stack to version 8.13.0, 8.13.1, or 8.13.2. This issue may go unnoticed unless you actively monitor your {{kib}} logs. The following log indicates the presence of this issue:

```shell
"params invalid: [anomalyDetectorTypes]: expected value of type [array] but got [undefined]"
```

This issue occurs because a non-optional parameter, `anomalyDetectorTypes` was added in 8.13.0 without the presence of an automation migration script. This breaks pre-existing rules as they do not have this parameter and will fail validation. This issue is fixed in v8.13.3.

There are three ways to fix this error:

* Upgrade to version 8.13.3
* Fix broken anomaly rules in the APM UI (no upgrade required)
* Fix broken anomaly rules with Kibana APIs (no upgrade required)

**Fix broken anomaly rules in the APM UI**

1. From any APM page in Kibana, select **Alerts and rules** → **Manage rules**.
2. Filter your rules by setting **Type** to **APM Anomaly**.
3. For each anomaly rule in the list, select the pencil icon to edit the rule.
4. Add one or more **DETECTOR TYPES** to the rule.

    The detector type determines when the anomaly rule triggers. For example, a latency anomaly rule will trigger when the latency of the service being monitored is abnormal. Supported detector types are `latency`, `throughput`, and `failed transaction rate`.

5. Click **Save**.

**Fix broken anomaly rules with Kibana APIs**

1. Find broken rules

    :::::{note}
    To identify rules in this exact state, you can use the [find rules endpoint](https://www.elastic.co/docs/api/doc/kibana/v8/group/endpoint-alerting) and search for the APM anomaly rule type as well as this exact error message indicating that the rule is in the broken state. We will also use the `fields` parameter to specify only the fields required when making the update request later.

    * `search_fields=alertTypeId`
    * `search=apm.anomaly`
    * `filter=alert.attributes.executionStatus.error.message:"params invalid: [anomalyDetectorTypes]: expected value of type [array] but got [undefined]"`
    * `fields=[id, name, actions, tags, schedule, notify_when, throttle, params]`

    The encoded request might look something like this:

    ```shell
    curl -u "$KIBANA_USER":"$KIBANA_PASSWORD" "$KIBANA_URL/api/alerting/rules/_find?search_fields=alertTypeId&search=apm.anomaly&filter=alert.attributes.executionStatus.error.message%3A%22params%20invalid%3A%20%5BanomalyDetectorTypes%5D%3A%20expected%20value%20of%20type%20%5Barray%5D%20but%20got%20%5Bundefined%5D%22&fields=id&fields=name&fields=actions&fields=tags&fields=schedule&fields=notify_when&fields=throttle&fields=params"
    ```

    ::::{dropdown} Example result:
    ```json
    {
      "page": 1,
      "total": 1,
      "per_page": 10,
      "data": [
        {
          "id": "d85e54de-f96a-49b5-99d4-63956f90a6eb",
          "name": "APM Anomaly Jason Test FAILING [2]",
          "tags": [
            "test",
            "jasonrhodes"
          ],
          "throttle": null,
          "schedule": {
            "interval": "1m"
          },
          "params": {
            "windowSize": 30,
            "windowUnit": "m",
            "anomalySeverityType": "warning",
            "environment": "ENVIRONMENT_ALL"
          },
          "notify_when": null,
          "actions": []
        }
      ]
    }
    ```

    ::::


    :::::

2. Prepare the update JSON doc(s)

    ::::{note}
    For each broken rule found, create a JSON rule document with what was returned from the API in the previous step. You will need to make two changes to each document:

    1. Remove the `id` key but keep the value connected to this document (e.g. rename the file to `{{id}}.json`). **The `id` cannot be sent as part of the request body for the PUT request, but you will need it for the URL path.**
    2. Add the `"anomalyDetectorTypes"` to the `"params"` block, using the default value as seen below to mimic the pre-8.13 behavior:

        ```json
        {
          "params": {
            // ... other existing params should stay here,
            // with the required one added to this object
            "anomalyDetectorTypes": [
              "txLatency",
              "txThroughput",
              "txFailureRate"
            ]
          }
        }
        ```


    ::::

3. Update each rule using the `PUT /api/alerting/rule/{{id}}` API

    ::::{note}
    For each rule, submit a PUT request to the [update rule endpoint](https://www.elastic.co/docs/api/doc/kibana/v8/group/endpoint-alerting) using that rule’s ID and its stored update document from the previous step. For example, assuming the first broken rule’s ID is `046c0d4f`:

    ```shell
    curl -u "$KIBANA_USER":"$KIBANA_PASSWORD" -XPUT "$KIBANA_URL/api/alerting/rule/046c0d4f" -H 'Content-Type: application/json' -H 'kbn-xsrf: rule-update' -d @046c0d4f.json
    ```

    Once the PUT request executes successfully, the rule will no longer be broken.

    ::::

:::

:::{dropdown} Upgrading APM Server to 8.11+ might break event intake from older APM Java agents
*APM Server versions: >=8.11.0*<br> *Elastic APM Java agent versions: < 1.43.0*

If you are using APM Server (> v8.11.0) and the Elastic APM Java agent (< v1.43.0), the agent may be sending empty histogram metricsets.

In previous APM Server versions some data validation was not properly applied, leading the APM Server to accept empty histogram metricsets where it shouldn’t. This bug was fixed in the APM Server in 8.11.0.

The APM Java agent (< v1.43.0) was sending this kind of invalid data under certain circumstances. If you upgrade the APM Server to v8.11.0+ *without* upgrading the APM Java agent version, metricsets can be rejected by the APM Server and can result in additional error logs in the Java agent.

The fix is to upgrade the Elastic APM Java agent to a version >= 1.43.0. Find details in [elastic/apm-data#157](https://github.com/elastic/apm-data/pull/157).

:::

:::{dropdown} traces-apm@custom ingest pipeline applied to certain data streams unintentionally
*APM Server versions: 8.12.0*<br>

If you’re using the Elastic APM Server v8.12.0, the `traces-apm@custom` ingest pipeline is now additionally applied to data streams `traces-apm.sampled-*` and `traces-apm.rum-*`, and applied twice for `traces-apm-*`. This bug impacts users with a non-empty `traces-apm@custom` ingest pipeline.

If you rely on this unintended behavior in 8.12.0, please rename your pipeline to `traces-apm.integration@custom` to preserve this behavior in later versions.

A fix was released in 8.12.1: [elastic/kibana#175448](https://github.com/elastic/kibana/pull/175448).

:::

:::{dropdown} Ingesting new JVM metrics in 8.9 and 8.10 breaks upgrade to 8.11 and stops ingestion
*APM Server versions: 8.11.0, 8.11.1*<br> *Elastic APM Java agent versions: 1.39.0+*

If you’re using the Elastic APM Java agent v1.39.0+ to send new JVM metrics to APM Server v8.9.x and v8.10.x, upgrading to 8.11.0 or 8.11.1 will silently fail and stop ingesting APM metrics.

After upgrading, you will see the following errors:

* APM Server error logs:

    ```txt
    failed to index document in 'metrics-apm.internal-default' (fail_processor_exception): Document produced by APM Server v8.11.1, which is newer than the installed APM integration (v8.10.3-preview-1695284222). The APM integration must be upgraded.
    ```

* Fleet error on integration package upgrade:

    ```txt
    Failed installing package [apm] due to error: [ResponseError: mapper_parsing_exception
    	Root causes:
    		mapper_parsing_exception: Field [jvm.memory.non_heap.pool.committed] attempted to shadow a time_series_metric]
    ```


A fix was released in 8.11.2: [elastic/kibana#171712](https://github.com/elastic/kibana/pull/171712).

:::

:::{dropdown} APM integration package upgrade through Fleet causes excessive data stream rollovers
*APM Server versions: <= 8.12.1 +*

If you’re upgrading APM integration package to any versions <= 8.12.1, in some rare cases, the upgrade fails with a mapping conflict error. The upgrade process keeps rolling over the data stream in an unsuccessful attempt to work around the error. As a result, many empty backing indices for APM data streams are created.

During upgrade, you will see errors similar to the one below:

* Fleet error on integration package upgrade:

    ```txt
    Mappings update for metrics-apm.service_destination.10m-default failed due to ResponseError: illegal_argument_exception
    	Root causes:
    		illegal_argument_exception: Mapper for [metricset.interval] conflicts with existing mapper:
    	Cannot update parameter [value] from [10m] to [null]
    ```


A fix was released in 8.12.2: [elastic/apm-server#12219](https://github.com/elastic/apm-server/pull/12219).

:::

:::{dropdown} Performance regression: APM issues too many small bulk requests for Elasticsearch output
*APM Server versions: >=8.13.0, <= 8.14.2*<br>

If you’re on APM server version >=8.13.0, <= 8.14.2_, using Elasticsearch output, do not specify any `output.elasticsearch.flush_bytes`, and do not disable compression explicitly by setting `output.elasticsearch.compression_level` to `0`, APM server will issue smaller bulk requests of 24KB size, and more bulk requests will need to be made to maintain the original throughput. This causes Elasticsearch to experience higher load, and APM server may exhibit Elasticsearch backpressure symptoms.

This happens because a performance regression was introduced, such that the default value of bulk indexer flush bytes was reduced from 1MB to 24KB.

Affected APM servers will emit the following log:

```txt
flush_bytes config value is too small (0) and might be ignored by the indexer, increasing value to 24576
```

To work around the issue, modify the Elasticsearch output configuration in APM.

* For APM Server binary

    * In `apm-server.yml`, set `output.elasticsearch.flush_bytes: 1mib`

* For Fleet-managed APM (non-Elastic Cloud)

    * In Fleet, open the Settings tab.
    * Under Outputs, identify the Elasticsearch output that receives from APM, select the edit icon.
    * In the Edit output flyout, in "Advanced YAML configuration" field, add line `flush_bytes: 1mib`.

* For Elastic Cloud

    * It is not possible to edit the Fleet "Elastic Cloud internal output".


A fix will be released in 8.14.3: [elastic/apm-server#13576](https://github.com/elastic/apm-server/pull/13576).

:::
