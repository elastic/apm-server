// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregation

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.elastic.co/apm/module/apmotel/v2"
	"go.elastic.co/apm/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"

	"github.com/elastic/apm-aggregation/aggregators"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/paths"
)

type Aggregator struct {
	logger         *zap.Logger
	baseaggregator *aggregators.Aggregator
}

// NewAggregator returns a new instance of aggregator.
func NewAggregator(
	ctx context.Context,
	maxSvcs int, maxTxGroups int,
	nextProcessor modelpb.BatchProcessor,
	logger *logp.Logger,
	tracer *apm.Tracer,
) (*Aggregator, error) {
	zapLogger := zap.New(logger.Core(), zap.WithCaller(true)).Named("aggregator")
	storageDir := paths.Resolve(paths.Data, "aggregator")
	if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
		return nil, err
	}

	tp, err := apmotel.NewTracerProvider()
	if err != nil {
		return nil, fmt.Errorf("failed creating apmotel tracer provider: %w", err)
	}
	otel.SetTracerProvider(tp)

	exporter, err := apmotel.NewGatherer()
	if err != nil {
		return nil, fmt.Errorf("failed creating apmotel gatherer: %w", err)
	}
	mp := metric.NewMeterProvider(metric.WithReader(exporter))
	otel.SetMeterProvider(mp)
	tracer.RegisterMetricsGatherer(exporter)

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
		aggregators.WithMeter(mp.Meter("aggregator")),
		aggregators.WithTracer(tp.Tracer("aggregator")),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create base aggregator: %w", err)
	}
	agg := &Aggregator{
		logger:         zapLogger,
		baseaggregator: baseaggregator,
	}
	return agg, err
}

// Run runs all the components of aggregator.
func (a *Aggregator) Run() error {
	return a.baseaggregator.Run(context.TODO())
}

// Stop stops all the component for aggregator.
func (a *Aggregator) Stop(ctx context.Context) error {
	err := a.baseaggregator.Stop(ctx)
	if err != nil {
		return fmt.Errorf("failed to stop aggregator: %w", err)
	}
	return nil
}

// ProcessBatch implements modelpb.BatchProcessor interface
// so that aggregator can consume events from intake.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	return a.baseaggregator.AggregateBatch(ctx, "", b)
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
