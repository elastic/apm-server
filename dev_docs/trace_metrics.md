# Trace metrics

APM Server aggregates several types of metrics from trace events: transaction
metrics, and service destination metrics. This process is sometimes referred
to as "pre-aggregation": aggregation of events prior to indexing, and indexing
the aggregated metrics, as opposed to aggregation of documents in Elasticsearch.

From 8.7.0 onwards, all aggregated metrics are aggregated at 3
different granularity: `1m`, `10m`, and `60m`.

## Transaction metrics

Transaction metrics measure the latency distribution for transaction groups.
As transactions are observed by APM Server, it groups them according to various
attributes such as `service.name`, `transaction.name`, and `kubernetes.pod.name`.
The latency is then recorded in an [HDRHistogram](http://hdrhistogram.org/) for
that group. Transaction group latency histograms are periodically indexed (every
minute by default), with configurable precision (defaults to 2 significant figures).

To protect against memory exhaustion due to high-cardinality transaction names
(or other attributes), at any given time, APM Server places a limit on the number
of services tracked, the number of transaction groups tracked, as well as number
of groups tracked per service.

By default, the limits are 1,000 services per GB of memory, 5,000 transaction groups
per GB of memory. When transaction group latency histograms are indexed, the groups
are reset, enabling a different set of groups to be recorded.
The per-service limit is 10% of the global limit. For example, for a 2GB APM Server,
the limits are 2,000 services, 10,000 transaction groups, and for each service,
there can be a maximum of 1,000 unique transaction groups.

## Service transaction metrics

Service transaction metrics are similar to Transaction metrics, but with fewer
dimensions. For example, `transaction.name` is no longer considered during aggregation.

A limit of 1,000 unique service transaction groups per GB of memory is enforced.

## Service destination metrics

Service destination metrics measure throughput and average latency of operations
from one service to another. This works much the same as transaction metrics
aggregation: span events describing an operation that involves another service
are grouped by the originating and target services, and the span latency is
accumulated. For these metrics we record only a count and sum, enabling calculation
of throughput and average latency. A default limit of 10,000 groups is
imposed.

## Service summary metrics

Service summary metrics consider transaction, error, log, and metric events and
basically produce a summary of all services sending events.

A limit of 1,000 unique service summary groups per GB of memory is enforced.

## Dealing with sampling

The APM app in Kibana visualises transaction throughput, both as an overall
throughput and as a histogram of transaction counts bucketed by latency. In the
past we powered these metrics by recording a document in Elasticsearch for every
transaction, sampled or not. We now use transaction metrics. The specific details
of the queries are documented at
https://github.com/elastic/kibana/blob/main/x-pack/plugins/apm/dev_docs/apm_queries.md

Transaction metrics are aggregated from transaction events. To avoid agents
having to send events to APM Server for non-sampled traces, we instead have
agents include the sampling rate in events that it does send. APM Server then
multiplies metrics by the inverse of the sampling rate, such that the recorded
metrics are scaled to approximate the complete population of traces. Lower
sampling rates may lead to greater statistical error when there is significant
variance.

For example, if a transaction is sent to APM Server with a sampling rate of 0.1,
then APM Server will increment the transaction metrics by a count of 10, each
having the same latency. In APM Server, the inverse sampling rate is recorded
on the in-memory trace event model as the field "RepresentativeCount".

As of version 8.0, APM Server discards all non-sampled transaction documents,
and agents may detect the server version and choose not to send non-sampled
transaction events.

Agents must propagate the sampling rate from the root transaction throughout
all downstream transactions and spans. This is accomplished by including the
sampling rate in the W3C Trace-State header, as described in
https://github.com/elastic/apm/blob/main/specs/agents/tracing-sampling.md.
Older agents, from before the Trace-State propagation was implemented, will
not include the sampling rate in events. In this case, APM Server will treat
each transaction event equally, i.e. assuming a sampling rate of 1. Only
service destination metrics are not measured in this case, as span events have
never been sent for non-sampled traces.

### OpenTelemetry

OpenTelemetry SDKs do not currently propagate enough information for us to be
able to extrapolate metrics. There is a proposal to propagate the information
through Trace-State at
https://opentelemetry.io/docs/reference/specification/trace/tracestate-probability-sampling/

As a result, all throughput metrics will be incorrect when sampling is used.
