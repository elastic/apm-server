// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package telemetry holds the logic for emitting telemetry when performing aggregation.
package telemetry

import (
	"context"
	"fmt"

	"github.com/cockroachdb/pebble"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	bytesUnit    = "by"
	countUnit    = "1"
	durationUnit = "s"
)

// Metrics are a collection of metric used to record all the
// measurements for the aggregators. Sync metrics are exposed
// and used by the calling code to record measurements whereas
// async instruments (mainly pebble database metrics) are
// collected by the observer pattern by passing a metrics provider.
type Metrics struct {
	// Synchronous metrics used to record aggregation measurements.

	EventsProcessed   metric.Float64Counter
	BytesProcessed    metric.Int64Counter
	MinQueuedDelay    metric.Float64Histogram
	ProcessingLatency metric.Float64Histogram
	MetricsOverflowed metric.Int64Counter

	// Asynchronous metrics used to get pebble metrics and
	// record measurements. These are kept unexported as they are
	// supposed to be updated via the registered callback.

	pebbleFlushes                  metric.Int64ObservableCounter
	pebbleFlushedBytes             metric.Int64ObservableCounter
	pebbleCompactions              metric.Int64ObservableCounter
	pebbleIngestedBytes            metric.Int64ObservableCounter
	pebbleCompactedBytesRead       metric.Int64ObservableCounter
	pebbleCompactedBytesWritten    metric.Int64ObservableCounter
	pebbleMemtableTotalSize        metric.Int64ObservableGauge
	pebbleTotalDiskUsage           metric.Int64ObservableGauge
	pebbleReadAmplification        metric.Int64ObservableGauge
	pebbleNumSSTables              metric.Int64ObservableGauge
	pebbleTableReadersMemEstimate  metric.Int64ObservableGauge
	pebblePendingCompaction        metric.Int64ObservableGauge
	pebbleMarkedForCompactionFiles metric.Int64ObservableGauge
	pebbleKeysTombstones           metric.Int64ObservableGauge

	// registration represents the token for a the configured callback.
	registration metric.Registration
}

type pebbleProvider func() *pebble.Metrics

// NewMetrics returns a new instance of the metrics.
func NewMetrics(provider pebbleProvider, opts ...Option) (*Metrics, error) {
	var err error
	var i Metrics

	cfg := newConfig(opts...)
	meter := cfg.Meter

	// Aggregator metrics
	i.EventsProcessed, err = meter.Float64Counter(
		"events.processed.count",
		metric.WithDescription("Number of processed APM Events. Dimensions are used to report the outcome"),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for events processed: %w", err)
	}
	i.BytesProcessed, err = meter.Int64Counter(
		"events.processed.bytes",
		metric.WithDescription("Number of bytes processed by the aggregators"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for bytes processed: %w", err)
	}
	i.ProcessingLatency, err = meter.Float64Histogram(
		"events.processed.latency",
		metric.WithDescription("Records the processing delays, removes expected delays due to aggregation intervals"),
		metric.WithUnit(durationUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for processing delay: %w", err)
	}
	i.MinQueuedDelay, err = meter.Float64Histogram(
		"events.processed.queued-latency",
		metric.WithDescription("Records total duration for aggregating a batch w.r.t. its youngest member"),
		metric.WithUnit(durationUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for queued delay: %w", err)
	}
	i.MetricsOverflowed, err = meter.Int64Counter(
		"metrics.overflowed.count",
		metric.WithDescription(
			"Estimated number of metric aggregation keys that resulted in an overflow, per interval and aggregation type",
		),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for metrics overflowed: %w", err)
	}

	// Pebble metrics
	i.pebbleFlushes, err = meter.Int64ObservableCounter(
		"pebble.flushes",
		metric.WithDescription("Number of memtable flushes to disk"),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for flushes: %w", err)
	}
	i.pebbleFlushedBytes, err = meter.Int64ObservableCounter(
		"pebble.flushed-bytes",
		metric.WithDescription("Bytes written during flush"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for flushed bytes: %w", err)
	}
	i.pebbleCompactions, err = meter.Int64ObservableCounter(
		"pebble.compactions",
		metric.WithDescription("Number of table compactions"),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for compactions: %w", err)
	}
	i.pebbleIngestedBytes, err = meter.Int64ObservableCounter(
		"pebble.ingested-bytes",
		metric.WithDescription("Bytes ingested"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for ingested bytes: %w", err)
	}
	i.pebbleCompactedBytesRead, err = meter.Int64ObservableCounter(
		"pebble.compacted-bytes-read",
		metric.WithDescription("Bytes read during compaction"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for compacted bytes read: %w", err)
	}
	i.pebbleCompactedBytesWritten, err = meter.Int64ObservableCounter(
		"pebble.compacted-bytes-written",
		metric.WithDescription("Bytes written during compaction"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for compacted bytes written: %w", err)
	}
	i.pebbleMemtableTotalSize, err = meter.Int64ObservableGauge(
		"pebble.memtable.total-size",
		metric.WithDescription("Current size of memtable in bytes"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for memtable size: %w", err)
	}
	i.pebbleTotalDiskUsage, err = meter.Int64ObservableGauge(
		"pebble.disk.usage",
		metric.WithDescription("Total disk usage by pebble, including live and obsolete files"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for total disk usage: %w", err)
	}
	i.pebbleReadAmplification, err = meter.Int64ObservableGauge(
		"pebble.read-amplification",
		metric.WithDescription("Current read amplification for the db"),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for read amplification: %w", err)
	}
	i.pebbleNumSSTables, err = meter.Int64ObservableGauge(
		"pebble.num-sstables",
		metric.WithDescription("Current number of storage engine SSTables"),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for count of sstables: %w", err)
	}
	i.pebbleTableReadersMemEstimate, err = meter.Int64ObservableGauge(
		"pebble.table-readers-mem-estimate",
		metric.WithDescription("Memory used by index and filter blocks"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for table cache readers: %w", err)
	}
	i.pebblePendingCompaction, err = meter.Int64ObservableGauge(
		"pebble.estimated-pending-compaction",
		metric.WithDescription("Estimated pending compaction bytes"),
		metric.WithUnit(bytesUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for pending compaction: %w", err)
	}
	i.pebbleMarkedForCompactionFiles, err = meter.Int64ObservableGauge(
		"pebble.marked-for-compaction-files",
		metric.WithDescription("Count of SSTables marked for compaction"),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for compaction marked files: %w", err)
	}
	i.pebbleKeysTombstones, err = meter.Int64ObservableGauge(
		"pebble.keys.tombstone.count",
		metric.WithDescription("Approximate count of delete keys across the storage engine"),
		metric.WithUnit(countUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric for tombstones: %w", err)
	}

	if err := i.registerCallback(meter, provider); err != nil {
		return nil, fmt.Errorf("failed to register callback: %w", err)
	}
	return &i, nil
}

// CleanUp unregisters any registered callback for collecting async
// measurements.
func (i *Metrics) CleanUp() error {
	if i == nil || i.registration == nil {
		return nil
	}
	if err := i.registration.Unregister(); err != nil {
		return fmt.Errorf("failed to unregister callback: %w", err)
	}
	return nil
}

func (i *Metrics) registerCallback(meter metric.Meter, provider pebbleProvider) (err error) {
	i.registration, err = meter.RegisterCallback(func(ctx context.Context, obs metric.Observer) error {
		pm := provider()
		obs.ObserveInt64(i.pebbleMemtableTotalSize, int64(pm.MemTable.Size))
		obs.ObserveInt64(i.pebbleTotalDiskUsage, int64(pm.DiskSpaceUsage()))

		obs.ObserveInt64(i.pebbleFlushes, pm.Flush.Count)
		obs.ObserveInt64(i.pebbleFlushedBytes, int64(pm.Levels[0].BytesFlushed))

		obs.ObserveInt64(i.pebbleCompactions, pm.Compact.Count)
		obs.ObserveInt64(i.pebblePendingCompaction, int64(pm.Compact.EstimatedDebt))
		obs.ObserveInt64(i.pebbleMarkedForCompactionFiles, int64(pm.Compact.MarkedFiles))

		obs.ObserveInt64(i.pebbleTableReadersMemEstimate, pm.TableCache.Size)
		obs.ObserveInt64(i.pebbleKeysTombstones, int64(pm.Keys.TombstoneCount))

		lm := pm.Total()
		obs.ObserveInt64(i.pebbleNumSSTables, lm.NumFiles)
		obs.ObserveInt64(i.pebbleIngestedBytes, int64(lm.BytesIngested))
		obs.ObserveInt64(i.pebbleCompactedBytesRead, int64(lm.BytesRead))
		obs.ObserveInt64(i.pebbleCompactedBytesWritten, int64(lm.BytesCompacted))
		obs.ObserveInt64(i.pebbleReadAmplification, int64(lm.Sublevels))
		return nil
	},
		i.pebbleMemtableTotalSize,
		i.pebbleTotalDiskUsage,
		i.pebbleFlushes,
		i.pebbleFlushedBytes,
		i.pebbleCompactions,
		i.pebbleIngestedBytes,
		i.pebbleCompactedBytesRead,
		i.pebbleCompactedBytesWritten,
		i.pebbleReadAmplification,
		i.pebbleNumSSTables,
		i.pebbleTableReadersMemEstimate,
		i.pebblePendingCompaction,
		i.pebbleMarkedForCompactionFiles,
		i.pebbleKeysTombstones,
	)
	return
}

// WithSuccess returns an attribute representing a successful event outcome.
func WithSuccess() attribute.KeyValue {
	return WithOutcome("success")
}

// WithFailure returns an attribute representing a failed event outcome.
func WithFailure() attribute.KeyValue {
	return WithOutcome("failure")
}

// WithOutcome returns an attribute for event outcome.
func WithOutcome(outcome string) attribute.KeyValue {
	return attribute.String("outcome", outcome)
}
