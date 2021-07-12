// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

// This file is mandatory as otherwise the apm-server.test binary is not generated correctly.

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pkg/errors"
	"go.elastic.co/apm/apmtest"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
	"github.com/elastic/beats/v7/libbeat/paths"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/model/modelprocessor"
)

var systemTest *bool

func init() {
	testing.Init()
	systemTest = flag.Bool("systemTest", false, "Set to true when running system tests")

	rootCmd.PersistentFlags().AddGoFlag(flag.CommandLine.Lookup("systemTest"))
	rootCmd.PersistentFlags().AddGoFlag(flag.CommandLine.Lookup("test.coverprofile"))
}

// TestSystem calls the main function. This is used by system tests to run
// the apm-server while also capturing code coverage.
func TestSystem(t *testing.T) {
	if *systemTest {
		main()
	}
}

func TestMonitoring(t *testing.T) {
	// samplingMonitoringRegistry will be nil, as under normal circumstances
	// we rely on apm-server/sampling to create the registry.
	samplingMonitoringRegistry = monitoring.NewRegistry()

	var aggregationMonitoringSnapshot, tailSamplingMonitoringSnapshot monitoring.FlatSnapshot
	runServerError := errors.New("runServer")
	runServer := func(ctx context.Context, args beater.ServerParams) error {
		aggregationMonitoringSnapshot = monitoring.CollectFlatSnapshot(aggregationMonitoringRegistry, monitoring.Full, false)
		tailSamplingMonitoringSnapshot = monitoring.CollectFlatSnapshot(samplingMonitoringRegistry, monitoring.Full, false)
		return runServerError
	}
	runServer = wrapRunServer(runServer)

	home := t.TempDir()
	err := paths.InitPaths(&paths.Path{Home: home})
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Aggregation.Transactions.Enabled = true
	cfg.Sampling.Tail.Enabled = true
	cfg.Sampling.Tail.Policies = []config.TailSamplingPolicy{{SampleRate: 0.1}}

	// Call the wrapped runServer twice, to ensure metric registration does not panic.
	for i := 0; i < 2; i++ {
		err := runServer(context.Background(), beater.ServerParams{
			Config:         cfg,
			Logger:         logp.NewLogger(""),
			Tracer:         apmtest.DiscardTracer,
			BatchProcessor: modelprocessor.Nop{},
			Managed:        true,
			Namespace:      "default",
		})
		assert.Equal(t, runServerError, err)
		assert.NotEqual(t, monitoring.MakeFlatSnapshot(), aggregationMonitoringSnapshot)
		assert.NotEqual(t, monitoring.MakeFlatSnapshot(), tailSamplingMonitoringSnapshot)
	}
}
