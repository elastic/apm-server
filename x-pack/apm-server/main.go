// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/gofrs/uuid/v5"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/x-pack/libbeat/management"
	"github.com/elastic/elastic-agent-client/v7/pkg/client"
	"github.com/elastic/elastic-agent-client/v7/pkg/proto"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/paths"
	"github.com/elastic/elastic-agent-libs/transport/tlscommon"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
	"github.com/elastic/apm-server/internal/beatcmd"
	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

const (
	tailSamplingStorageDir = "tail_sampling"
)

var (
	// Note: this registry is created in github.com/elastic/apm-server/sampling. That package
	// will hopefully disappear in the future, when agents no longer send unsampled transactions.
	samplingMonitoringRegistry = monitoring.Default.GetRegistry("apm-server.sampling")

	// badgerDB holds the badger database to use when tail-based sampling is configured.
	badgerMu sync.Mutex
	badgerDB *eventstorage.StorageManager

	storageMu sync.Mutex
	storage   *eventstorage.ManagedReadWriter

	// samplerUUID is a UUID used to identify sampled trace ID documents
	// published by this process.
	samplerUUID = uuid.Must(uuid.NewV4())
)

func init() {
	tlscommon.SetInsecureDefaults()

	management.ConfigTransform.SetTransform(
		func(unit *proto.UnitExpectedConfig, agentInfo *client.AgentInfo) ([]*reload.ConfigWithMeta, error) {
			// NOTE(axw) we intentionally do not log the entire config here,
			// as it may contain secrets (Elasticsearch API Key, secret token).
			logger := logp.NewLogger("")
			logger.With(
				logp.String("agent.id", agentInfo.ID),
				logp.String("agent.version", agentInfo.Version),
				logp.String("unit.id", unit.Id),
				logp.Uint64("unit.revision", unit.Revision),
			).Info("received input from elastic-agent")

			cfg, err := config.NewConfigFrom(unit.GetSource().AsMap())
			if err != nil {
				return nil, err
			}
			return []*reload.ConfigWithMeta{{Config: cfg}}, nil
		},
	)
}

type namedProcessor struct {
	processor
	name string
}

type processor interface {
	modelpb.BatchProcessor
	Run() error
	Stop(context.Context) error
}

// newProcessors returns a list of processors which will process
// events in sequential order, prior to the events being published.
func newProcessors(args beater.ServerParams) ([]namedProcessor, error) {
	var processors []namedProcessor

	aggregationProcessor, err := newAggregationProcessor(args)
	if err != nil {
		return nil, err
	}
	processors = append(processors, aggregationProcessor)

	if args.Config.Sampling.Tail.Enabled {
		const name = "tail sampler"
		sampler, err := newTailSamplingProcessor(args)
		if err != nil {
			return nil, fmt.Errorf("error creating %s: %w", name, err)
		}
		samplingMonitoringRegistry.Remove("tail")
		monitoring.NewFunc(samplingMonitoringRegistry, "tail", sampler.CollectMonitoring, monitoring.Report)
		processors = append(processors, namedProcessor{name: name, processor: sampler})
	}
	return processors, nil
}

func newTailSamplingProcessor(args beater.ServerParams) (*sampling.Processor, error) {
	tailSamplingConfig := args.Config.Sampling.Tail
	es, err := args.NewElasticsearchClient(tailSamplingConfig.ESConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client for tail-sampling: %w", err)
	}

	storageDir := paths.Resolve(paths.Data, tailSamplingStorageDir)
	badgerDB, err = getBadgerDB(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get Badger database: %w", err)
	}
	readWriter := getStorage(badgerDB)

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
		BatchProcessor: args.BatchProcessor,
		LocalSamplingConfig: sampling.LocalSamplingConfig{
			FlushInterval:         tailSamplingConfig.Interval,
			MaxDynamicServices:    1000,
			Policies:              policies,
			IngestRateDecayFactor: tailSamplingConfig.IngestRateDecayFactor,
		},
		RemoteSamplingConfig: sampling.RemoteSamplingConfig{
			CompressionLevel: tailSamplingConfig.ESConfig.CompressionLevel,
			Elasticsearch:    es,
			SampledTracesDataStream: sampling.DataStreamConfig{
				Type:      "traces",
				Dataset:   "apm.sampled",
				Namespace: args.Namespace,
			},
			UUID: samplerUUID.String(),
		},
		StorageConfig: sampling.StorageConfig{
			DB:                    badgerDB,
			Storage:               readWriter,
			StorageDir:            storageDir,
			StorageGCInterval:     tailSamplingConfig.StorageGCInterval,
			StorageLimit:          tailSamplingConfig.StorageLimitParsed,
			TTL:                   tailSamplingConfig.TTL,
			DiscardOnWriteFailure: tailSamplingConfig.DiscardOnWriteFailure,
		},
	})
}

func getBadgerDB(storageDir string) (*eventstorage.StorageManager, error) {
	badgerMu.Lock()
	defer badgerMu.Unlock()
	if badgerDB == nil {
		sm, err := eventstorage.NewStorageManager(storageDir)
		if err != nil {
			return nil, err
		}
		badgerDB = sm
	}
	return badgerDB, nil
}

func getStorage(sm *eventstorage.StorageManager) *eventstorage.ManagedReadWriter {
	storageMu.Lock()
	defer storageMu.Unlock()
	if storage == nil {
		storage = sm.NewReadWriter()
	}
	return storage
}

// runServerWithProcessors runs the APM Server and the given list of processors.
//
// newProcessors returns a list of processors which will process events in
// sequential order, prior to the events being published.
func runServerWithProcessors(ctx context.Context, runServer beater.RunServerFunc, args beater.ServerParams, processors ...namedProcessor) error {
	if len(processors) == 0 {
		return runServer(ctx, args)
	}

	g, ctx := errgroup.WithContext(ctx)
	serverStopped := make(chan struct{})
	for _, p := range processors {
		p := p // copy for closure
		g.Go(func() error {
			if err := p.Run(); err != nil {
				args.Logger.With(logp.Error(err)).Errorf("%s aborted", p.name)
				return err
			}
			args.Logger.Infof("%s stopped", p.name)
			return nil
		})
		g.Go(func() error {
			<-serverStopped
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
		defer close(serverStopped)
		return runServer(ctx, args)
	})
	return g.Wait()
}

func wrapServer(args beater.ServerParams, runServer beater.RunServerFunc) (beater.ServerParams, beater.RunServerFunc, error) {
	processors, err := newProcessors(args)
	if err != nil {
		return beater.ServerParams{}, nil, err
	}

	// Add the processors to the chain.
	processorChain := make(modelprocessor.Chained, len(processors)+1)
	for i, p := range processors {
		processorChain[i] = p
	}
	processorChain[len(processors)] = args.BatchProcessor
	args.BatchProcessor = processorChain

	wrappedRunServer := func(ctx context.Context, args beater.ServerParams) error {
		return runServerWithProcessors(ctx, runServer, args, processors...)
	}
	return args, wrappedRunServer, nil
}

// closeBadger is called at process exit time to close the badger.DB opened
// by the tail-based sampling processor constructor, if any. This is never
// called concurrently with opening badger.DB/accessing the badgerDB global,
// so it does not need to hold badgerMu.
func closeBadger() error {
	if badgerDB != nil {
		return badgerDB.Close()
	}
	return nil
}

func cleanup() error {
	return closeBadger()
}

func Main() error {
	rootCmd := newXPackRootCommand(
		func(args beatcmd.RunnerParams) (beatcmd.Runner, error) {
			return beater.NewRunner(beater.RunnerParams{
				Config:     args.Config,
				Logger:     args.Logger,
				WrapServer: wrapServer,

				MeterProvider:   args.MeterProvider,
				MetricsGatherer: args.MetricsGatherer,
			})
		},
	)
	result := rootCmd.Execute()
	cleanupErr := cleanup()
	return errors.Join(result, cleanupErr)
}

func main() {
	if err := Main(); err != nil {
		os.Exit(1)
	}
}
