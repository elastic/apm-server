# Tail-based sampling

APM Server has a tail-based sampling feature which samples whole traces after they have completed,
based on properties such as the trace name, outcome, and duration.

Our tail-based sampling implementation works in a partially-distributed manner. Each APM Server will
buffer trace events locally, make sampling decisions for the trace roots that it observes, and publish
those to Elasticsearch. Each APM Server will subscribe to decisions made by others, and index related
events that it has buffered locally.

## Criteria

Users can define criteria for matching traces, based on properties available in root transactions:

- root service name
- root service environment
- trace name (root transaction name)
- trace outcome (root transaction outcome)

Users can define multiple "policies", each of which comprises a set of properties to match, and a
sampling rate. Policies are matched in the order defined.

In the future we may extend tail-based sampling to allow decisions to be made on non-root trace
events, e.g. based on the occurrence of exceptions within a backend service. If we do this, we will
have to disable a current optimisation: we currently avoid buffering trace events in the APM Server
that processes a root transaction which is not added to a sampling reservoir (see next section).
If sampling decisions for a trace may be made in any APM Server, on non-root events, we will have to
buffer all events all the time.

## Weighted sampling

For tail-based sampling, APM Server uses a weighted random reservoir sampling algorithm known as
[Algorithm A-Res](https://en.wikipedia.org/wiki/Reservoir_sampling#Algorithm_A-Res). At a high level,
this works by maintaining a "reservoir" to which transactions are added and removed as they are
observed; the decision to add and remove transactions is random, and weighted based on transaction
latency. By weighting on latency, we prefer to keep slower transactions, under the assumption that
slower transactions are the more interesting ones.

As root transactions are received by an APM Server, it will first identify its sampling group by
matching the transaction to a policy; each sampling group has its own sampling reservoir. These
reservoirs are periodically published to Elasticsearch, cleared, and resized according to the ingest
rate of transactions matching that policy.

## Coherency/coordination

Like head-based sampling, where a decision is made at the trace root and propagated throughout by
agents, tail-based sampling is coherent: all events for a trace are sampled consistently as a whole.

Because sampling decisions are made only at the trace root, and not all trace events are necessarily
received by one APM Server, we must propagate sampling decisions made by each APM Server to each of
the others. We do this by publishing decisions to Elasticsearch, in a special "sampled traces" data
stream with a short retention time, and have all APM Servers poll the data stream for new documents.

APM Servers, and hence events for one trace, may be spread across multiple Elasticsearch clusters.
At present tail-based sampling does not have cross-cluster support, but in the future we intend to
support this by enabling APM Servers to poll sampled traces data streams in multiple clusters.

## Local buffering

As mentioned above, APM Servers will buffer events locally until a sampling decision has been made.
For this purpose, we use [Pebble](https://github.com/cockroachdb/pebble) as the local event storage database. Pebble is designed for
fast writes, which is important for our use case: we write all events here, and read out only the
sampled ones. All non-sampled events will be removed from the store through TTL expiry implemented in APM Server.

Event writes to the local database may stop to avoid filling the disk.
By default, storage limit is `0` which makes APM Server align its disk usage with the disk size.
APM server uses up to 80% of the disk size limit on the disk where the local tail-based sampling database is located.
The last 20% of disk will not be used by APM Server.
It is the recommended value as it automatically scales with the disk size.
Alternatively, a concrete GB value can be set for the maximum amount of disk used for tail-based sampling.

When event writes fail, or the storage limit has been reached, the default behavior is to sample
all incoming traces. This behavior is configurable and allow users to either discard
or sample incoming events when the disk is full.
