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
	"github.com/elastic/beats/v7/libbeat/monitoring"
	"github.com/elastic/beats/v7/libbeat/paths"
	"github.com/elastic/beats/v7/x-pack/libbeat/licenser"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/spanmetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/txmetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/cmd"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
)

var (
	aggregationMonitoringRegistry = monitoring.Default.NewRegistry("apm-server.aggregation")
)

type namedProcessor struct {
	name string
	processor
}

type processor interface {
	ProcessTransformables(context.Context, []transform.Transformable) ([]transform.Transformable, error)
	Run() error
	Stop(context.Context) error
}

// newProcessors returns a list of processors which will process
// events in sequential order, prior to the events being published.
func newProcessors(args beater.ServerParams) ([]namedProcessor, error) {
	var processors []namedProcessor
	if args.Config.Aggregation.Transactions.Enabled {
		const name = "transaction metrics aggregation"
		args.Logger.Infof("creating %s with config: %+v", name, args.Config.Aggregation.Transactions)
		agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
			Report:                         args.Reporter,
			MaxTransactionGroups:           args.Config.Aggregation.Transactions.MaxTransactionGroups,
			MetricsInterval:                args.Config.Aggregation.Transactions.Interval,
			HDRHistogramSignificantFigures: args.Config.Aggregation.Transactions.HDRHistogramSignificantFigures,
			RUMUserAgentLRUSize:            args.Config.Aggregation.Transactions.RUMUserAgentLRUSize,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "error creating %s", name)
		}
		processors = append(processors, namedProcessor{name: name, processor: agg})
		monitoring.NewFunc(aggregationMonitoringRegistry, "txmetrics", agg.CollectMonitoring, monitoring.Report)
	}
	if args.Config.Aggregation.ServiceDestinations.Enabled {
		const name = "service destinations aggregation"
		args.Logger.Infof("creating %s with config: %+v", name, args.Config.Aggregation.ServiceDestinations)
		spanAggregator, err := spanmetrics.NewAggregator(spanmetrics.AggregatorConfig{
			Report:    args.Reporter,
			Interval:  args.Config.Aggregation.ServiceDestinations.Interval,
			MaxGroups: args.Config.Aggregation.ServiceDestinations.MaxGroups,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "error creating %s", name)
		}
		processors = append(processors, namedProcessor{name: name, processor: spanAggregator})
	}
	if args.Config.Sampling.Tail != nil && args.Config.Sampling.Tail.Enabled {
		const name = "tail sampler"
		sampler, err := newTailSamplingProcessor(args)
		if err != nil {
			return nil, errors.Wrapf(err, "error creating %s", name)
		}
		processors = append(processors, namedProcessor{name: name, processor: sampler})
	}
	return processors, nil
}

func newTailSamplingProcessor(args beater.ServerParams) (*sampling.Processor, error) {
	// Tail-based sampling is a Platinum-licensed feature.
	//
	// FIXME(axw) each time licenser.Enforce is called an additional global
	// callback is registered with Elasticsearch, which fetches the license
	// and checks it. The root command already calls licenser.Enforce with
	// licenser.BasicAndAboveOrTrial. We need to make this overridable to
	// avoid redundant checks.
	checkPlatinum := licenser.CheckLicenseCover(licenser.Platinum)
	licenser.Enforce("apm-server", func(logger *logp.Logger, license licenser.License) bool {
		logger.Infof("Checking license for tail-based sampling")
		return checkPlatinum(logger, license) || licenser.CheckTrial(logger, license)
	})

	tailSamplingConfig := args.Config.Sampling.Tail
	es, err := elasticsearch.NewClient(tailSamplingConfig.ESConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Elasticsearch client for tail-sampling")
	}
	policies := make([]sampling.Policy, len(tailSamplingConfig.Policies))
	for i, in := range tailSamplingConfig.Policies {
		policies[i] = sampling.Policy{
			PolicyCriteria: sampling.PolicyCriteria{
				ServiceName:        in.Service.Name,
				ServiceEnvironment: in.Service.Environment,
				TraceName:          in.Trace.Name,
				TraceOutcome:       in.Trace.Outcome,
			},
			SampleRate: in.SampleRate,
		}
	}
	return sampling.NewProcessor(sampling.Config{
		BeatID:   args.Info.ID.String(),
		Reporter: args.Reporter,
		LocalSamplingConfig: sampling.LocalSamplingConfig{
			FlushInterval: tailSamplingConfig.Interval,
			// TODO(axw) make MaxDynamicServices configurable?
			MaxDynamicServices:    1000,
			Policies:              policies,
			IngestRateDecayFactor: tailSamplingConfig.IngestRateDecayFactor,
		},
		RemoteSamplingConfig: sampling.RemoteSamplingConfig{
			Elasticsearch: es,
			// TODO(axw) make index name configurable?
			SampledTracesIndex: "apm-sampled-traces",
		},
		StorageConfig: sampling.StorageConfig{
			StorageDir:        paths.Resolve(paths.Data, tailSamplingConfig.StorageDir),
			StorageGCInterval: tailSamplingConfig.StorageGCInterval,
			TTL:               tailSamplingConfig.TTL,
		},
	})
}

// runServerWithProcessors runs the APM Server and the given list of processors.
//
// newProcessors returns a list of processors which will process events in
// sequential order, prior to the events being published.
func runServerWithProcessors(ctx context.Context, runServer beater.RunServerFunc, args beater.ServerParams, processors ...namedProcessor) error {
	if len(processors) == 0 {
		return runServer(ctx, args)
	}

	origReport := args.Reporter
	args.Reporter = func(ctx context.Context, req publish.PendingReq) error {
		var err error
		for _, p := range processors {
			req.Transformables, err = p.ProcessTransformables(ctx, req.Transformables)
			if err != nil {
				return err
			}
		}
		return origReport(ctx, req)
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, p := range processors {
		p := p // copy for closure
		g.Go(func() error {
			if err := p.Run(); err != nil {
				args.Logger.Errorf("%s aborted", p.name, logp.Error(err))
				return err
			}
			args.Logger.Infof("%s stopped", p.name)
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
			return p.Stop(stopctx)
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
			processors, err := newProcessors(args)
			if err != nil {
				return err
			}
			return runServerWithProcessors(ctx, runServer, args, processors...)
		}
	},
}))

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
