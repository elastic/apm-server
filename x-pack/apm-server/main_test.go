// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

// This file is mandatory as otherwise the apm-server.test binary is not generated correctly.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.elastic.co/apm/v2/apmtest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-agent-libs/paths"

	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/pubsub/pubsubtest"
)

func TestMonitoring(t *testing.T) {
	home := t.TempDir()
	err := paths.InitPaths(&paths.Path{Home: home})
	require.NoError(t, err)
	defer closeBadger() // close badger.DB so data dir can be deleted on Windows

	cfg := config.DefaultConfig()
	cfg.Sampling.Tail.Enabled = true
	cfg.Sampling.Tail.Policies = []config.TailSamplingPolicy{{SampleRate: 0.1}}
	// MaxServices and MaxGroups are configured based on memory limit.
	// Overriding here to avoid validation errors.
	cfg.Aggregation.MaxServices = 10000
	cfg.Aggregation.Transactions.MaxGroups = 10000
	cfg.Aggregation.ServiceTransactions.MaxGroups = 10000
	cfg.Aggregation.ServiceDestinations.MaxGroups = 10000

	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Wrap & run the server twice, to ensure metric registration does not panic.
	runServerError := errors.New("runServer")
<<<<<<< HEAD
=======
	runServerFunc := func(ctx context.Context, args beater.ServerParams) error {
		// run server for some time until storage metrics are reported by the storage manager
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			monitoringtest.ExpectContainOtelMetricsKeys(c, reader, []string{
				"apm-server.sampling.tail.storage.lsm_size",
				"apm-server.sampling.tail.storage.value_log_size",
			})
		}, time.Second, 10*time.Millisecond)

		return runServerError
	}
>>>>>>> ed9f4622 (test(TestMonitoring): pass the correct collect struct to helper method (#18692))
	for i := 0; i < 2; i++ {
		serverParams, runServer, err := wrapServer(beater.ServerParams{
			Config:                 cfg,
			Logger:                 logptest.NewTestingLogger(t, ""),
			Tracer:                 apmtest.DiscardTracer,
			MeterProvider:          mp,
			BatchProcessor:         modelpb.ProcessBatchFunc(func(ctx context.Context, b *modelpb.Batch) error { return nil }),
			Namespace:              "default",
			NewElasticsearchClient: elasticsearch.NewClient,
		}, func(ctx context.Context, args beater.ServerParams) error {
			return runServerError
		})
		require.NoError(t, err)

		err = runServer(context.Background(), serverParams)
		assert.Equal(t, runServerError, err)
		monitoringtest.ExpectOtelMetrics(t, reader, map[string]any{
			"apm-server.sampling.tail.storage.lsm_size":       0,
			"apm-server.sampling.tail.storage.value_log_size": 0,
		})
	}
}

func TestStorageMonitoring(t *testing.T) {
	config, reader := newTempdirConfig(t)

	processor, err := sampling.NewProcessor(config, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)
	go processor.Run()
	defer processor.Stop(context.Background())
	for i := 0; i < 100; i++ {
		traceID := uuid.Must(uuid.NewV4()).String()
		batch := modelpb.Batch{{
			Trace: &modelpb.Trace{Id: traceID},
			Event: &modelpb.Event{Duration: uint64(123 * time.Millisecond)},
			Transaction: &modelpb.Transaction{
				Type:    "type",
				Id:      traceID,
				Sampled: true,
			},
		}}
		err := processor.ProcessBatch(context.Background(), &batch)
		require.NoError(t, err)
		assert.Empty(t, batch)
	}

	// Stop the processor, flushing pending writes, and reopen storage.
	// Reopening storage is necessary to immediately recalculate the
	// storage size, otherwise we must wait for a minute (hard-coded in
	// badger) for storage metrics to be updated.
	err = processor.Stop(context.Background())
	require.NoError(t, err)
	require.NoError(t, closeBadger())
	badgerDB, err = getBadgerDB(config.StorageDir, config.MeterProvider)
	require.NoError(t, err)

	lsmSize := getGauge(t, reader, "apm-server.sampling.tail.storage.lsm_size")
	assert.NotZero(t, lsmSize)
	vlogSize := getGauge(t, reader, "apm-server.sampling.tail.storage.value_log_size")
	assert.NotZero(t, vlogSize)
}

func newTempdirConfig(tb testing.TB) (sampling.Config, sdkmetric.Reader) {
	tempdir := filepath.Join(tb.TempDir(), "samplingtest")
	reader := sdkmetric.NewManualReader(sdkmetric.WithTemporalitySelector(
		func(ik sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.DeltaTemporality
		},
	))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	closeBadger()
	badgerDB, err := getBadgerDB(tempdir, mp)
	require.NoError(tb, err)
	tb.Cleanup(func() { closeBadger() })

	storage := badgerDB.NewReadWriter()

	return sampling.Config{
		BatchProcessor: modelpb.ProcessBatchFunc(func(context.Context, *modelpb.Batch) error { return nil }),
		MeterProvider:  mp,
		LocalSamplingConfig: sampling.LocalSamplingConfig{
			FlushInterval:         time.Second,
			MaxDynamicServices:    1000,
			IngestRateDecayFactor: 0.9,
			Policies: []sampling.Policy{
				{SampleRate: 0.1},
			},
		},
		RemoteSamplingConfig: sampling.RemoteSamplingConfig{
			Elasticsearch: pubsubtest.Client(nil, nil),
			SampledTracesDataStream: sampling.DataStreamConfig{
				Type:      "traces",
				Dataset:   "sampled",
				Namespace: "testing",
			},
			UUID: "local-apm-server",
		},
		StorageConfig: sampling.StorageConfig{
			DB:                badgerDB,
			Storage:           storage,
			StorageDir:        tempdir,
			StorageGCInterval: time.Second,
			TTL:               30 * time.Minute,
			StorageLimit:      0, // No storage limit.
		},
	}, reader
}

func getGauge(t testing.TB, reader sdkmetric.Reader, name string) int64 {
	var rm metricdata.ResourceMetrics
	assert.NoError(t, reader.Collect(context.Background(), &rm))

	assert.NotEqual(t, 0, len(rm.ScopeMetrics))

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m.Data.(metricdata.Gauge[int64]).DataPoints[0].Value
			}
		}
	}

	return 0
}
