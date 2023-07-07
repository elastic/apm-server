// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package aggregation

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.elastic.co/apm/module/apmzap/v2"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-aggregation/aggregators"
	"github.com/elastic/apm-data/model/modelpb"
)

type Aggregator struct {
	logger         *zap.Logger
	baseaggregator *aggregators.Aggregator
}

// NewAggregator returns a new instance of aggregator.
func NewAggregator(
	ctx context.Context,
	nextProcessor modelpb.BatchProcessor,
) (*Aggregator, error) {
	// TODO(carsonip): Respect apm-server log config
	var apmzapCore apmzap.Core
	encoderCfg := ecszap.NewDefaultEncoderConfig()
	level := zap.NewAtomicLevelAt(zapcore.DebugLevel)
	core := ecszap.NewCore(encoderCfg, os.Stderr, level)
	logger := zap.New(apmzapCore.WrapCore(core), zap.AddCaller())
	// TODO(carsonip): Where should we store the db files? Should it be configurable?
	dir, err := os.MkdirTemp("", "lsm")
	if err != nil {
		return nil, err
	}
	baseaggregator, err := aggregators.New(
		aggregators.WithDataDir(dir),
		aggregators.WithLimits(aggregators.Limits{
			MaxSpanGroups:                         1000,
			MaxSpanGroupsPerService:               100,
			MaxTransactionGroups:                  1000,
			MaxTransactionGroupsPerService:        100,
			MaxServiceTransactionGroups:           1000,
			MaxServiceTransactionGroupsPerService: 100,
			MaxServiceInstanceGroupsPerService:    100,
			MaxServices:                           1000,
		}),
		aggregators.WithProcessor(wrapNextProcessor(nextProcessor)),
		aggregators.WithAggregationIntervals([]time.Duration{time.Second, time.Minute, time.Hour}),
		aggregators.WithLogger(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create base aggregator: %w", err)
	}
	agg := &Aggregator{
		logger:         logger,
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
