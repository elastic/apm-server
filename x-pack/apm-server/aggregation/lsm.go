// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregation

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/elastic/apm-aggregation/aggregators"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/paths"
)

type Aggregator struct {
	logger         *zap.Logger
	baseaggregator *aggregators.Aggregator
	writerPool     sync.Pool

	stopOnce sync.Once
	stopped  chan struct{}
}

// NewAggregator returns a new instance of aggregator.
func NewAggregator(
	ctx context.Context,
	maxSvcs int, maxTxGroups int,
	nextProcessor modelpb.BatchProcessor,
	logger *logp.Logger,
) (*Aggregator, error) {
	zapLogger := zap.New(logger.Core(), zap.WithCaller(true)).Named("aggregator")
	storageDir := paths.Resolve(paths.Data, "aggregator")
	if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
		return nil, err
	}

	baseaggregator, err := aggregators.New(
		aggregators.WithDataDir(storageDir),
		aggregators.WithLimits(aggregators.Limits{
			MaxSpanGroups:                         10000,
			MaxSpanGroupsPerService:               1000,
			MaxTransactionGroups:                  maxTxGroups,
			MaxTransactionGroupsPerService:        maxTxGroups / 10,
			MaxServiceTransactionGroups:           maxSvcs,
			MaxServiceTransactionGroupsPerService: maxSvcs / 10,
			MaxServiceInstanceGroupsPerService:    maxSvcs / 10,
			MaxServices:                           maxSvcs,
		}),
		aggregators.WithProcessor(wrapNextProcessor(nextProcessor)),
		aggregators.WithAggregationIntervals([]time.Duration{time.Minute, 10 * time.Minute, time.Hour}),
		aggregators.WithLogger(zapLogger),
		aggregators.WithMeter(otel.GetMeterProvider().Meter("aggregator")),
		aggregators.WithTracer(otel.GetTracerProvider().Tracer("aggregator")),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create base aggregator: %w", err)
	}
	agg := &Aggregator{
		logger:         zapLogger,
		baseaggregator: baseaggregator,
		stopped:        make(chan struct{}),
	}
	return agg, err
}

// Run runs all the components of aggregator.
func (a *Aggregator) Run() error {
	if err := a.baseaggregator.StartHarvesting(); err != nil {
		return err
	}
	<-a.stopped
	return nil
}

// Stop stops all the component for aggregator.
func (a *Aggregator) Stop(ctx context.Context) error {
	err := a.baseaggregator.Close(ctx)
	if err != nil {
		return fmt.Errorf("failed to stop aggregator: %w", err)
	}
	a.stopOnce.Do(func() { close(a.stopped) })
	return nil
}

// ProcessBatch implements modelpb.BatchProcessor interface
// so that aggregator can consume events from intake.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	writer, ok := a.writerPool.Get().(*aggregators.Writer)
	if !ok {
		var err error
		writer, err = a.baseaggregator.NewWriter()
		if err != nil {
			return fmt.Errorf("error creating aggregation writer: %w", err)
		}
	}
	defer a.writerPool.Put(writer)
	return writer.WriteEventMetrics(ctx, [16]byte{}, (*b)...)
}

func wrapNextProcessor(processor modelpb.BatchProcessor) aggregators.Processor {
	return func(
		ctx context.Context,
		cmk aggregators.CombinedMetricsKey,
		cm aggregators.CombinedMetrics,
		aggregationIvl time.Duration,
	) error {
		batch, err := aggregators.CombinedMetricsToBatch(cm, cmk.ProcessingTime, aggregationIvl)
		if err != nil {
			return fmt.Errorf("failed to convert combined metrics to batch: %w", err)
		}
		if err := processor.ProcessBatch(
			ctx,
			batch,
		); err != nil {
			return fmt.Errorf("failed to process batch: %w", err)
		}
		return nil
	}
}
