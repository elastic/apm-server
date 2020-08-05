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
func runServerWithAggregator(ctx context.Context, runServer beater.RunServerFunc, args beater.ServerParams) error {
	if !args.Config.Aggregation.Enabled {
		return runServer(ctx, args)
	}

	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		Report:                         args.Reporter,
		MaxTransactionGroups:           args.Config.Aggregation.MaxTransactionGroups,
		MetricsInterval:                args.Config.Aggregation.Interval,
		HDRHistogramSignificantFigures: args.Config.Aggregation.HDRHistogramSignificantFigures,
		RUMUserAgentLRUSize:            args.Config.Aggregation.RUMUserAgentLRUSize,
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
		if err := agg.Run(); err != nil {
			args.Logger.Errorf("aggregator aborted", logp.Error(err))
			return err
		}
		args.Logger.Infof("aggregator stopped")
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		stopctx := context.Background()
		if args.Config.ShutdownTimeout > 0 {
			// On shutdown wait for the aggregator to stop
			// in order to flush any accumulated metrics.
			var cancel context.CancelFunc
			stopctx, cancel = context.WithTimeout(stopctx, args.Config.ShutdownTimeout)
			defer cancel()
		}
		return agg.Stop(stopctx)
	})
	g.Go(func() error {
		return runServer(ctx, args)
	})
	return g.Wait()
}

var rootCmd = cmd.NewXPackRootCommand(beater.NewCreator(beater.CreatorParams{
	WrapRunServer: func(runServer beater.RunServerFunc) beater.RunServerFunc {
		return func(ctx context.Context, args beater.ServerParams) error {
			return runServerWithAggregator(ctx, runServer, args)
		}
	},
}))

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
