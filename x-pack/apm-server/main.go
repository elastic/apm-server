// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/txmetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/cmd"
)

// runServerWithAggregator runs the APM Server. If aggregation
// is enabled, then a txmetrics.Aggregator will also be run,
// and the publish.Reporter will be wrapped such that all
// transactions pass through the aggregator before being
// published to libbeat.
func runServerWithAggregator(ctx context.Context, args beater.ServerParams) error {
	if !args.Config.Aggregation.Enabled {
		return beater.RunServer(ctx, args)
	}

	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		Report:                         args.Reporter,
		MaxTransactionGroups:           args.Config.Aggregation.MaxTransactionGroups,
		MetricsInterval:                args.Config.Aggregation.Interval,
		HDRHistogramSignificantFigures: args.Config.Aggregation.HDRHistogramSignificantFigures,
	})
	if err != nil {
		return errors.Wrap(err, "error creating aggregator")
	}

	origReport := args.Reporter
	args.Reporter = func(ctx context.Context, req publish.PendingReq) error {
		req.Transformables = agg.AggregateTransformables(req.Transformables)
		return origReport(ctx, req)
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		args.Logger.Infof("aggregator started with config: %+v", args.Config.Aggregation)
		switch err := agg.Run(ctx); err {
		case nil, context.Canceled:
			args.Logger.Infof("aggregator stopped")
			return nil
		default:
			args.Logger.Errorf("aggregator aborted", logp.Error(err))
			return err
		}
	})
	g.Go(func() error {
		return beater.RunServer(ctx, args)
	})
	return g.Wait()
}

var rootCmd = cmd.NewXPackRootCommand(beater.NewCreator(beater.CreatorParams{
	RunServer: func(ctx context.Context, args beater.ServerParams) error {
		return runServerWithAggregator(ctx, args)
	},
}))

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
