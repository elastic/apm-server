# APM Integration

The APM integration installs Elasticsearch templates and ingest node pipelines for APM data.

To learn more about the APM Integration architecture, see [APM Components](https://ela.st/apm-components).

### Quick start

Ready to jump in? Read the [APM quick start](https://ela.st/quick-start-apm).

### How to use this integration

Add the APM integration to an Elastic Agent policy to create an `apm` input.
Any Elastic Agents set up with this policy will run an APM Server binary locally.
Don't forget to configure the APM Server `host`, especially if it needs to be accessed from outside, like when running in Docker.
Then, configure your APM agents to communicate with APM Server.

If you have Real User Monitoring (RUM) enabled, you must run Elastic Agent centrally.
Otherwise, you can run it on edge machines by downloading and installing Elastic Agent
on the same machines that your instrumented services run.

#### Data Streams

When using the APM integration, apm events are indexed into data streams. Data stream names contain the event type,
service name, and a user-configurable namespace.

There is no specific recommendation for what to use as a namespace; it is intentionally flexible.
You might use the environment, like `production`, `testing`, or `development`,
or you could namespace data by business unit. It is your choice.

See [APM data streams](https://ela.st/apm-data-streams) for more information.

## Compatibility

The APM integration requires Kibana and Elasticsearch `7.12.x`+ with at least the basic license.
The APM integration version should match the Elastic Stack Major.Minor version. For example,
the APM integration version `7.16.2` should be run with the Elastic Stack `7.16.x`.

IMPORTANT: If you run APM Server with Elastic Agent manually in standalone mode, you must install the APM integration,
otherwise the APM Server will not ingest any events.

## Traces

Traces are comprised of [spans and transactions](https://www.elastic.co/guide/en/apm/get-started/current/apm-data-model.html).

Traces are written to `traces-apm-*` data streams, except for RUM traces, which are written to `traces-apm.rum-*`.

{{fields "traces"}}

## Application Metrics

Application metrics are comprised of custom, application-specific metrics, basic system metrics such as CPU and memory usage,
and runtime metrics such as JVM garbage collection statistics.

Application metrics are written to service-specific `metrics-apm.app.*-*` data streams.

{{fields "app_metrics"}}

## Internal Metrics

Internal metrics comprises metrics produced by Elastic APM agents and Elastic APM server for powering various Kibana charts
in the APM app, such as "Time spent by span type".

Internal metrics are written to `metrics-apm.internal-*` data streams.

{{fields "internal_metrics"}}

## Application errors

Application errors comprises error/exception events occurring in an application.

Application errors are written to `logs-apm.error.*` data stream.

{{fields "error_logs"}}
