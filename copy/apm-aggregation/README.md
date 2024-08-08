# apm-aggregation

APM metrics aggregation library that implements an LSM (Log Structured Merge tree)-based metrics aggregator.

Files are subject to Elastic License v2. See LICENSE.txt for more.

## Instrumentation

`apm-aggregation` uses OTEL to instrument itself. Instrumentation produces a set
of metrics to help monitor the status of aggregations. This section describes the
metrics produced by `apm-aggregation` in detail.

### Instrumentation Areas

`apm-aggregation` aggregates metrics using LSM based key-value store [pebble](https://github.com/cockroachdb/pebble).
The intrumentation covers two broad areas:

1. The core aggregation logic, including ingestion and harvest.
2. Performance of pebble database.

### Metrics

`apm-aggregation` records and publishes the following metrics:

#### `events.processed.count`

- Type: `Float64Counter`

The number of processed APM Events. It includes successfully and unsuccessfully
processed events, which are reported as dimensions.

##### Dimensions

- [`combined_metrics_id`](#combined_metrics_id)
- [`aggregation_interval`](#aggregation_interval)
- [`outcome`](#outcome)

#### `events.processed.bytes`

- Type: `Int64Counter`

The number of encoded bytes processed by the aggregator. This reports the same number
of bytes that is written to the underlying db.

##### Dimensions

- [`combined_metrics_id`](#combined_metrics_id)
- [`outcome`](#outcome)

#### `events.processed.latency`

- Type: `Float64Histogram`

The processing delay for a batch of APM events accepted at a specific processing
time. It is recorded after removing any expected delays due to aggregation interval
or configuration.

##### Dimensions

- [`combined_metrics_id`](#combined_metrics_id)
- [`aggregation_interval`](#aggregation_interval)
- [`outcome`](#outcome)

#### `events.processed.queued-latency`

- Type: `Float64Histogram`

The delay in processing a batch based on the youngest APM event received in the batch.

##### Dimensions

- [`combined_metrics_id`](#combined_metrics_id)
- [`aggregation_interval`](#aggregation_interval)
- [`outcome`](#outcome)

#### `metrics.overflowed.count`

- Type: `Int64Counter`

Estimated number of metric aggregation keys that resulted in an overflow, per interval and aggregation type.

##### Dimensions

- [`combined_metrics_id`](#combined_metrics_id)
- [`aggregation_interval`](#aggregation_interval)
- [`aggregation_type`](#aggregation_type)

#### `pebble.flushes`

- Type: `Int64ObservableCounter`

The number of memtable flushes to disk.

#### `pebble.flushed-bytes`

- Type: `Int64ObservableCounter`

The number of bytes written during a flush.

#### `pebble.compactions`

- Type: `Int64ObservableCounter`

The number of table compactions performed by pebble.

#### `pebble.ingested-bytes`

- Type: `Int64ObservableCounter`

The number of bytes ingested by pebble.

#### `pebble.compacted-bytes-read`

- Type: `Int64ObservableCounter`

The number of bytes read during compaction.

#### `pebble.compacted-bytes-written`

- Type: `Int64ObservableCounter`

The number of bytes written during compaction.

#### `pebble.memtable.total-size`

- Type: `Int64ObservableGauge`

The current size of memtable in bytes.

#### `pebble.disk.usage`

- Type: `Int64ObservableGauge`

The current total disk usage by pebble in bytes, including live and obsolete files.

#### `pebble.read-amplification`

- Type: `Int64ObservableGauge`

The current read amplification for the db.

#### `pebble.num-sstables`

- Type: `Int64ObservableGauge`

The current number of SSTables.

#### `pebble.table-readers-mem-estimate`

- Type: `Int64ObservableGauge`

The memory in bytes used by pebble for index and fliter blocks.

#### `pebble.estimated-pending-compaction`

- Type: `Int64ObservableGauge`

The current number of estimated bytes pending for compaction.

#### `pebble.marked-for-compaction-files`

- Type: `Int64ObservableGauge`

The current number of SSTables marked for compaction.

#### `pebble.keys.tombstone.count`

- Type: `Int64ObservableGauge`

The approximate count of delete keys across the storage engine.

### Dimensions

This section lists the general dimensions published by some of the metric.

#### `combined_metrics_id`

This is an optional dimension. The key and the value of this dimension depends
on the option `WithCombinedMetricsIDToKVs` passed to the aggregator. If this
option is not supplied then this dimension is omitted.

#### `aggregation_interval`

Holds the value of aggregation interval for which the combined metrics is produced.
For example: `1m`, `10m`, etc.

#### `aggregation_type`

Holds the the aggregation type for which an overflow occurred.
For example: `service`, `transaction`, `service_transaction`, `service_destination`.

#### `outcome`

##### `success`

Events that have been successfully aggregated into the final combined metrics and
processed as part of the harvest.

##### `failure`

Events that failed to be aggregated for an reason and were dropped at any stage
in the aggregation process.
