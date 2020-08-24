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
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/spanmetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/txmetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/cmd"
)

// runServerWithAggregation runs the APM Server with additional transactions and spans aggregators
// if they are enabled. The publish.Reporter will be wrapped such that events pass through the
// configured aggregators.
func runServerWithAggregation(ctx context.Context, runServer beater.RunServerFunc, args beater.ServerParams) error {

	txMetricsAggregator, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		Report:                         args.Reporter,
		MaxTransactionGroups:           args.Config.Aggregation.Transactions.MaxTransactionGroups,
		MetricsInterval:                args.Config.Aggregation.Transactions.Interval,
		HDRHistogramSignificantFigures: args.Config.Aggregation.Transactions.HDRHistogramSignificantFigures,
		RUMUserAgentLRUSize:            args.Config.Aggregation.Transactions.RUMUserAgentLRUSize,
	})
	if err != nil {
		return errors.Wrap(err, "error creating transaction aggregator")
	}

	spanAggregator, err := spanmetrics.NewAggregator(spanmetrics.AggregatorConfig{
		Report:   args.Reporter,
		Interval: args.Config.Aggregation.ServiceDestinations.Interval,
	})
	if err != nil {
		return errors.Wrap(err, "error creating span aggregator")
	}

	origReport := args.Reporter
	args.Reporter = func(ctx context.Context, req publish.PendingReq) error {
		if args.Config.Aggregation.Transactions.Enabled {
			req.Transformables = txMetricsAggregator.AggregateTransformables(req.Transformables)
		}
		if args.Config.Aggregation.ServiceDestinations.Enabled {
			// TODO async?
			spanAggregator.AggregateTransformables(req.Transformables)
		}
		return origReport(ctx, req)
	}

	g, ctx := errgroup.WithContext(ctx)

	if args.Config.Aggregation.Transactions.Enabled {
		g.Go(func() error {
			args.Logger.Infof("transaction aggregator started with config: %+v", args.Config.Aggregation)
			if err := txMetricsAggregator.Run(); err != nil {
				args.Logger.Errorf("transaction aggregator aborted", logp.Error(err))
				return err
			}
			args.Logger.Infof("transaction aggregator stopped")
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
			return txMetricsAggregator.Stop(stopctx)
		})
	}

	if args.Config.Aggregation.ServiceDestinations.Enabled {
		g.Go(func() error {
			args.Logger.Infof("span aggregator started with config: %+v", args.Config.Aggregation)
			if err := spanAggregator.Run(); err != nil {
				args.Logger.Errorf("span aggregator aborted", logp.Error(err))
				return err
			}
			args.Logger.Infof("span aggregator stopped")
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
			return spanAggregator.Stop(stopctx)
		})
	}

	g.Go(func() error {
		return runServer(ctx, args)
	})

	return g.Wait()
}

var rootCmd = cmd.NewXPackRootCommand(beater.NewCreator(beater.CreatorParams{
	WrapRunServer: func(runServer beater.RunServerFunc) beater.RunServerFunc {
		return func(ctx context.Context, args beater.ServerParams) error {
			return runServerWithAggregation(ctx, runServer, args)
		}
	},
}))

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
