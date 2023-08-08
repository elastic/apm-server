// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregation

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/elastic/apm-aggregation/aggregationpb"
	"github.com/elastic/apm-aggregation/aggregators"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/elastic-agent-libs/logp"
)

type Aggregator struct {
	logger         *zap.Logger
	baseaggregator *aggregators.Aggregator
}

// New returns a new Aggregator.
func New(
	maxSvcs, maxTxGroups, maxSvcTxGroups, maxSpanGroups int,
	nextProcessor modelpb.BatchProcessor, logger *logp.Logger,
) (*Aggregator, error) {
	zapLogger := zap.New(logger.Core(), zap.WithCaller(true)).Named("aggregator")

	baseaggregator, err := aggregators.New(
		aggregators.WithLimits(aggregators.Limits{
			MaxSpanGroups:                         maxSpanGroups,
			MaxSpanGroupsPerService:               max(maxSpanGroups/10, 1),
			MaxTransactionGroups:                  maxTxGroups,
			MaxTransactionGroupsPerService:        max(maxTxGroups/10, 1),
			MaxServiceTransactionGroups:           maxSvcTxGroups,
			MaxServiceTransactionGroupsPerService: max(maxSvcTxGroups/10, 1),
			MaxServiceInstanceGroupsPerService:    max(maxSvcs/10, 1),
			MaxServices:                           maxSvcs,
		}),
		aggregators.WithProcessor(wrapNextProcessor(nextProcessor)),
		aggregators.WithAggregationIntervals([]time.Duration{time.Minute, 10 * time.Minute, time.Hour}),
		aggregators.WithLogger(zapLogger),
		aggregators.WithMeter(otel.GetMeterProvider().Meter("aggregator")),
		aggregators.WithTracer(otel.GetTracerProvider().Tracer("aggregator")),
		aggregators.WithInMemory(true),
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
	if err := a.baseaggregator.Run(context.Background()); err != aggregators.ErrAggregatorClosed {
		return err
	}
	return nil
}

// Stop stops all the component of aggregator.
func (a *Aggregator) Stop(ctx context.Context) error {
	err := a.baseaggregator.Close(ctx)
	if err != nil {
		return fmt.Errorf("failed to stop aggregator: %w", err)
	}
	return nil
}

// ProcessBatch implements modelpb.BatchProcessor interface
// so that aggregator can consume events from intake.
func (a *Aggregator) ProcessBatch(ctx context.Context, b *modelpb.Batch) error {
	for _, e := range *b {
		removeRUMGlobalLabels(e)
	}
	return a.baseaggregator.AggregateBatch(ctx, [16]byte{}, b)
}

func wrapNextProcessor(processor modelpb.BatchProcessor) aggregators.Processor {
	return func(
		ctx context.Context,
		cmk aggregators.CombinedMetricsKey,
		cm *aggregationpb.CombinedMetrics,
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

func removeRUMGlobalLabels(event *modelpb.APMEvent) {
	// Remove global labels for RUM services to avoid explosion of metric groups
	// to track for servicetxmetrics.
	// For consistency, this will remove labels for other aggregated metrics as well.
	switch event.GetAgent().GetName() {
	case "rum-js", "js-base", "android/java", "iOS/swift":
	default:
		return
	}

	// Setting the labels to non-global so that they are ignored by the aggregator.
	for _, v := range event.Labels {
		v.Global = false
	}
	for _, v := range event.NumericLabels {
		v.Global = false
	}
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
