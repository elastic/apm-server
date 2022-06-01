# Application metrics

APM Server supports receiving metrics from Elastic APM agents and OpenTelemetry SDKs.
These metrics can be categorised as either internal, or application metrics.

Internal metrics are those metrics captured by APM agents to power some part of the APM
app in Kibana, including transaction breakdown metrics, system/process CPU and memory
usage, and runtime metrics. These are all well-defined, and known to APM Server. They
are stored in the `metrics-apm.internal-<namespace>` data stream.

Application metrics are custom metrics defined by an application. For example, the Java
agent will send metrics measured with the popular [Micrometer](https://micrometer.io)
framework to APM Server. These metrics are not known to APM Server: they may have any
name the application developer chooses. Metrics may also have varying types: they might
be simple counters or gauges; or they might be more complex, such as histograms, or
summary statistics.

Because metric names are not known to APM Server, it must dynamically map them;
and because metrics may have different types, two metrics with the same name but
different types will lead to mapping conflicts. To avoid such mapping conflicts,
as well as to enable per-application retention policies, we send application metrics
to service-specific data streams: `metrics-apm.app.<service.name>-<namespace>`.
It is still possible for a single application to produce mapping conflicts by
changing the field type of a metric, but we expect this to be less likely.

## Dotted metric names

Metrics are indexed exactly as they are named, to ensure they are discoverable.
For example, say a histogram metric is named `requests.latency_distribution`, then
it will be indexed in Elasticsearch as:

```
{
  "requests": {
    "latency_distribution": {
      "counts": [...],
      "values": [...]
    }
  }
}
```

This can lead to issues where one metric is the prefix of another. Say you had
another counter metric, simply named `requests`. This would be indexed as:

```
{
  "requests": 123
}
```

In the latter metric, "requests" is a numerical field, while in the former,
"requests" is an object field. This scenario leads to a mapping conflict.
We have been working with the Elasticsearch team to improve on this: see
https://github.com/elastic/apm/issues/347. In the future, we expect to flatten
all metrics such that they are indexed like:

```
{
  "requests": 123,
  "requests.latency_distribution": {
    "counts": [...],
    "values": [...]
  }
}
```

In the mean time, some agents work around this issue by replacing dots
in metric names. For example, the Java agent will de-dot custom applications
metrics by default, providing the
[dedot_custom_metrics](https://www.elastic.co/guide/en/apm/agent/java/current/config-metrics.html#config-dedot-custom-metrics)
configuration option to opt-out.

## Dynamically mapping complex metrics

Unless otherwise instructed, Elasticsearch will dynamically map fields based
on the JSON data type: double are mapped to doubles, longs to longs,
strings to text with a .keyword subfield, etc. This is insufficient for
dynamically mapping complex field types, such as `histogram` and
`aggregate_metric_double`.

We could alternatively impose a naming convention, e.g. dynamically map all
`histogram.*` fields to histogram field types, which would require the previous
example of `requests.latency_histogram` to become something like
`histogram.requests.latency_distribution`. This has the undesirable outcome of
leaking implementation details to the user interface.

Instead, we use a feature of Elasticsearch that enables a bulk request or
ingest pipeline to specify the dynamic template to use for mapping a field.
For dynamically mapping histograms, we define a dynamic template named
"histogram"; we then have an ingest pipeline which directs Elasticsearch to
use this dynamic template for fields that have `"type": "histogram"`.
