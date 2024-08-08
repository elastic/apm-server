// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package telemetry

import (
	"context"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

func TestNewInstruments(t *testing.T) {
	expected := []metricdata.Metrics{
		{
			Name:        "pebble.flushes",
			Description: "Number of memtable flushes to disk",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
			},
		},
		{
			Name:        "pebble.flushed-bytes",
			Description: "Bytes written during flush",
			Unit:        "by",
			Data: metricdata.Sum[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
			},
		},
		{
			Name:        "pebble.compactions",
			Description: "Number of table compactions",
			Unit:        "1",
			Data: metricdata.Sum[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
			},
		},
		{
			Name:        "pebble.ingested-bytes",
			Description: "Bytes ingested",
			Unit:        "by",
			Data: metricdata.Sum[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
			},
		},
		{
			Name:        "pebble.compacted-bytes-read",
			Description: "Bytes read during compaction",
			Unit:        "by",
			Data: metricdata.Sum[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
			},
		},
		{
			Name:        "pebble.compacted-bytes-written",
			Description: "Bytes written during compaction",
			Unit:        "by",
			Data: metricdata.Sum[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
			},
		},
		{
			Name:        "pebble.memtable.total-size",
			Description: "Current size of memtable in bytes",
			Unit:        "by",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
		{
			Name:        "pebble.disk.usage",
			Description: "Total disk usage by pebble, including live and obsolete files",
			Unit:        "by",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
		{
			Name:        "pebble.read-amplification",
			Description: "Current read amplification for the db",
			Unit:        "1",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
		{
			Name:        "pebble.num-sstables",
			Description: "Current number of storage engine SSTables",
			Unit:        "1",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
		{
			Name:        "pebble.table-readers-mem-estimate",
			Description: "Memory used by index and filter blocks",
			Unit:        "by",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
		{
			Name:        "pebble.estimated-pending-compaction",
			Description: "Estimated pending compaction bytes",
			Unit:        "by",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
		{
			Name:        "pebble.marked-for-compaction-files",
			Description: "Count of SSTables marked for compaction",
			Unit:        "1",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
		{
			Name:        "pebble.keys.tombstone.count",
			Description: "Approximate count of delete keys across the storage engine",
			Unit:        "1",
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{Value: 0},
				},
			},
		},
	}

	rdr := metric.NewManualReader()
	meter := metric.NewMeterProvider(metric.WithReader(rdr)).Meter("test")
	instruments, err := NewMetrics(
		func() *pebble.Metrics { return &pebble.Metrics{} },
		WithMeter(meter),
	)

	require.NoError(t, err)
	require.NotNil(t, instruments)
	var rm metricdata.ResourceMetrics
	assert.NoError(t, rdr.Collect(context.Background(), &rm))

	require.Len(t, rm.ScopeMetrics, 1)
	sm := rm.ScopeMetrics[0]
	require.Len(t, sm.Metrics, len(expected))
	for i, em := range expected {
		metricdatatest.AssertEqual(t, em, sm.Metrics[i], metricdatatest.IgnoreTimestamp())
	}
}
