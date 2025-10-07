// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

// This file is mandatory as otherwise the apm-server.test binary is not generated correctly.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	"github.com/elastic/elastic-agent-libs/paths"

	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/monitoringtest"
	"github.com/elastic/apm-server/internal/elasticsearch"
)

func TestMonitoring(t *testing.T) {
	home := t.TempDir()
	err := paths.InitPaths(&paths.Path{Home: home})
	require.NoError(t, err)
	defer closeDB() // close DB so data dir can be deleted on Windows

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
	for i := 0; i < 2; i++ {
		serverParams, runServer, err := wrapServer(beater.ServerParams{
			Config:                 cfg,
			Logger:                 logptest.NewTestingLogger(t, ""),
			MeterProvider:          mp,
			BatchProcessor:         modelpb.ProcessBatchFunc(func(ctx context.Context, b *modelpb.Batch) error { return nil }),
			Namespace:              "default",
			NewElasticsearchClient: elasticsearch.NewClient,
		}, runServerFunc)
		require.NoError(t, err)

		err = runServer(context.Background(), serverParams)
		assert.Equal(t, runServerError, err)
	}
}
