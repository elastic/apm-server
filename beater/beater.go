// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package beater

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/common/transport"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/licenser"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/monitoring"
	esoutput "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"
	"github.com/elastic/go-ucfg"

	"github.com/elastic/apm-server/beater/config"
	javaattacher "github.com/elastic/apm-server/beater/java_attacher"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelindexer"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/sourcemap"
)

// CreatorParams holds parameters for creating beat.Beaters.
type CreatorParams struct {
	// Logger is a logger to use in Beaters created by the beat.Creator.
	//
	// If Logger is nil, logp.NewLogger will be used to create a new one.
	Logger *logp.Logger

	// WrapRunServer is used to wrap the RunServerFunc used to run the APM Server.
	//
	// WrapRunServer is optional. If provided, it must return a function that calls
	// its input, possibly modifying the parameters on the way in.
	WrapRunServer func(RunServerFunc) RunServerFunc
}

// NewCreator returns a new beat.Creator which creates beaters
// using the provided CreatorParams.
func NewCreator(args CreatorParams) beat.Creator {
	return func(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
		logger := args.Logger
		if logger != nil {
			logger = logger.Named(logs.Beater)
		} else {
			logger = logp.NewLogger(logs.Beater)
		}
		bt := &beater{
			rawConfig:                 ucfg,
			stopped:                   false,
			logger:                    logger,
			wrapRunServer:             args.WrapRunServer,
			waitPublished:             publish.NewWaitPublishedAcker(),
			outputConfigReloader:      newChanReloader(),
			libbeatMonitoringRegistry: monitoring.Default.GetRegistry("libbeat"),
		}

		var elasticsearchOutputConfig *common.Config
		if hasElasticsearchOutput(b) {
			elasticsearchOutputConfig = b.Config.Output.Config()
		}
		var err error
		bt.config, err = config.NewConfig(bt.rawConfig, elasticsearchOutputConfig)
		if err != nil {
			return nil, err
		}

		if bt.config.Pprof.Enabled {
			// Profiling rates should be set once, early on in the program.
			runtime.SetBlockProfileRate(bt.config.Pprof.BlockProfileRate)
			runtime.SetMutexProfileFraction(bt.config.Pprof.MutexProfileRate)
			if bt.config.Pprof.MemProfileRate > 0 {
				runtime.MemProfileRate = bt.config.Pprof.MemProfileRate
			}
		}

		if b.Manager != nil && b.Manager.Enabled() {
			// Subscribe to output changes for reconfiguring apm-server's Elasticsearch
			// clients, which use the Elasticsearch output config by default. We install
			// this during beat creation to ensure output config reloads are not missed;
			// reloads will be blocked until the chanReloader is served by beater.run.
			b.OutputConfigReloader = bt.outputConfigReloader
		}

		return bt, nil
	}
}

type beater struct {
	rawConfig                 *common.Config
	config                    *config.Config
	logger                    *logp.Logger
	wrapRunServer             func(RunServerFunc) RunServerFunc
	waitPublished             *publish.WaitPublishedAcker
	outputConfigReloader      *chanReloader
	libbeatMonitoringRegistry *monitoring.Registry

	mutex      sync.Mutex // guards stopServer and stopped
	stopServer func()
	stopped    bool
}

// Run runs the APM Server, blocking until the beater's Stop method is called,
// or a fatal error occurs.
func (bt *beater) Run(b *beat.Beat) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := bt.run(ctx, cancel, b); err != nil {
		return err
	}
	return bt.waitPublished.Wait(ctx)
}

func (bt *beater) run(ctx context.Context, cancelContext context.CancelFunc, b *beat.Beat) error {
	tracer, tracerServer, err := initTracing(b, bt.config, bt.logger)
	if err != nil {
		return err
	}
	if tracerServer != nil {
		defer tracerServer.Close()
	}
	if tracer != nil {
		defer tracer.Close()
	}

	reloader := reloader{
		runServerContext: ctx,
		args: sharedServerRunnerParams{
			Beat:                      b,
			WrapRunServer:             bt.wrapRunServer,
			Logger:                    bt.logger,
			Tracer:                    tracer,
			TracerServer:              tracerServer,
			Acker:                     bt.waitPublished,
			LibbeatMonitoringRegistry: bt.libbeatMonitoringRegistry,
		},
	}

	stopped := make(chan struct{})
	stopServer := func() {
		defer close(stopped)
		if bt.config.ShutdownTimeout > 0 {
			time.AfterFunc(bt.config.ShutdownTimeout, cancelContext)
		}
		reloader.stop()
	}
	if !bt.setStopServerFunc(stopServer) {
		// Server has already been stopped.
		stopServer()
		return nil
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		<-stopped
		return nil
	})
	if b.Manager != nil && b.Manager.Enabled() {
		// Managed by Agent: register input and output reloaders to reconfigure the server.
		reload.Register.MustRegisterList("inputs", &reloader)
		g.Go(func() error {
			return bt.outputConfigReloader.serve(
				ctx, reload.ReloadableFunc(reloader.reloadOutput),
			)
		})

		// Start the manager after all the hooks are initialized
		// and defined this ensure reloading consistency..
		if err := b.Manager.Start(); err != nil {
			return err
		}
		defer b.Manager.Stop()

	} else {
		// Management disabled, use statically defined config.
		reloader.rawConfig = bt.rawConfig
		if b.Config != nil {
			reloader.outputConfig = b.Config.Output
		}
		if err := reloader.reload(); err != nil {
			return err
		}
	}
	return g.Wait()
}

// setStopServerFunc sets a function to call when the server is stopped.
//
// setStopServerFunc returns false if the server has already been stopped.
func (bt *beater) setStopServerFunc(stopServer func()) bool {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()
	if bt.stopped {
		return false
	}
	bt.stopServer = stopServer
	return true
}

type reloader struct {
	runServerContext context.Context
	args             sharedServerRunnerParams

	mu           sync.Mutex
	namespace    string
	rawConfig    *common.Config
	outputConfig common.ConfigNamespace
	fleetConfig  *config.Fleet
	runner       *serverRunner
}

func (r *reloader) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runner != nil {
		r.runner.cancelRunServerContext()
		<-r.runner.done
		r.runner = nil
	}
}

// Reload is invoked when the initial, or updated, integration policy, is received.
func (r *reloader) Reload(configs []*reload.ConfigWithMeta) error {
	if n := len(configs); n != 1 {
		return fmt.Errorf("only 1 input supported, got %d", n)
	}
	cfg := configs[0]

	integrationConfig, err := config.NewIntegrationConfig(cfg.Config)
	if err != nil {
		return err
	}
	var namespace string
	if integrationConfig.DataStream != nil {
		namespace = integrationConfig.DataStream.Namespace
	}

	r.mu.Lock()
	r.namespace = namespace
	r.rawConfig = integrationConfig.APMServer
	r.fleetConfig = &integrationConfig.Fleet
	r.mu.Unlock()
	return r.reload()
}

func (r *reloader) reloadOutput(config *reload.ConfigWithMeta) error {
	var outputConfig common.ConfigNamespace
	if config != nil {
		if err := config.Config.Unpack(&outputConfig); err != nil {
			return err
		}
	}
	r.mu.Lock()
	r.outputConfig = outputConfig
	r.mu.Unlock()
	return r.reload()
}

func (r *reloader) reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rawConfig == nil {
		// APM Server config not loaded yet.
		return nil
	}

	runner, err := newServerRunner(r.runServerContext, serverRunnerParams{
		sharedServerRunnerParams: r.args,
		RawConfig:                r.rawConfig,
		FleetConfig:              r.fleetConfig,
		OutputConfig:             r.outputConfig,
	})
	if err != nil {
		return err
	}
	// Start listening before we stop the existing runner (if any), to ensure zero downtime.
	listener, err := listen(runner.config, runner.logger)
	if err != nil {
		return err
	}
	go func() {
		defer listener.Close()
		if err := runner.run(listener); err != nil {
			r.args.Logger.Error(err)
		}
	}()
	// If the old runner exists, cancel it
	if r.runner != nil {
		r.runner.cancelRunServerContext()
		<-r.runner.done
	}
	r.runner = runner
	return nil
}

type serverRunner struct {
	// backgroundContext is used for operations that should block on Stop,
	// up to the process shutdown timeout limit. This allows the publisher to
	// drain its queue when the server is stopped, for example.
	backgroundContext context.Context

	// runServerContext is used for the runServer call, and will be cancelled
	// immediately when the Stop method is invoked.
	runServerContext       context.Context
	cancelRunServerContext context.CancelFunc
	done                   chan struct{}

	pipeline                  beat.PipelineConnector
	acker                     *publish.WaitPublishedAcker
	namespace                 string
	config                    *config.Config
	rawConfig                 *common.Config
	elasticsearchOutputConfig *common.Config
	fleetConfig               *config.Fleet
	beat                      *beat.Beat
	logger                    *logp.Logger
	tracer                    *apm.Tracer
	tracerServer              *tracerServer
	wrapRunServer             func(RunServerFunc) RunServerFunc
	libbeatMonitoringRegistry *monitoring.Registry
}

type serverRunnerParams struct {
	sharedServerRunnerParams

	RawConfig    *common.Config
	FleetConfig  *config.Fleet
	OutputConfig common.ConfigNamespace
}

type sharedServerRunnerParams struct {
	Beat                      *beat.Beat
	WrapRunServer             func(RunServerFunc) RunServerFunc
	Logger                    *logp.Logger
	Tracer                    *apm.Tracer
	TracerServer              *tracerServer
	Acker                     *publish.WaitPublishedAcker
	LibbeatMonitoringRegistry *monitoring.Registry
}

func newServerRunner(ctx context.Context, args serverRunnerParams) (*serverRunner, error) {
	var esOutputConfig *common.Config
	if args.OutputConfig.Name() == "elasticsearch" {
		esOutputConfig = args.OutputConfig.Config()
	}

	cfg, err := config.NewConfig(args.RawConfig, esOutputConfig)
	if err != nil {
		return nil, err
	}

	runServerContext, cancel := context.WithCancel(ctx)
	return &serverRunner{
		backgroundContext:      ctx,
		runServerContext:       runServerContext,
		cancelRunServerContext: cancel,
		done:                   make(chan struct{}),

		config:                    cfg,
		rawConfig:                 args.RawConfig,
		elasticsearchOutputConfig: esOutputConfig,
		fleetConfig:               args.FleetConfig,
		acker:                     args.Acker,
		pipeline:                  args.Beat.Publisher,
		namespace:                 cfg.DataStreams.Namespace,
		beat:                      args.Beat,
		logger:                    args.Logger,
		tracer:                    args.Tracer,
		tracerServer:              args.TracerServer,
		wrapRunServer:             args.WrapRunServer,
		libbeatMonitoringRegistry: args.LibbeatMonitoringRegistry,
	}, nil
}

func (s *serverRunner) run(listener net.Listener) error {
	defer close(s.done)

	// Send config to telemetry.
	recordAPMServerConfig(s.config)

	var kibanaClient kibana.Client
	if s.config.Kibana.Enabled {
		kibanaClient = kibana.NewConnectingClient(&s.config.Kibana)
	}

	cfg := ucfg.Config(*s.rawConfig)
	parentCfg := cfg.Parent()
	// Check for an environment variable set when running in a cloud environment
	eac := os.Getenv("ELASTIC_AGENT_CLOUD")
	if eac != "" && s.config.Kibana.Enabled {
		// Don't block server startup sending the config.
		go func() {
			if err := kibana.SendConfig(s.runServerContext, kibanaClient, parentCfg); err != nil {
				s.logger.Infof("failed to upload config to kibana: %v", err)
			}
		}()
	}

	if s.config.JavaAttacherConfig.Enabled {
		if eac == "" {
			// We aren't running in a cloud environment
			go func() {
				attacher, err := javaattacher.New(s.config.JavaAttacherConfig)
				if err != nil {
					s.logger.Errorf("failed to start java attacher: %v", err)
					return
				}
				if err := attacher.Run(s.runServerContext); err != nil {
					s.logger.Errorf("failed to run java attacher: %v", err)
				}
			}()
		} else {
			s.logger.Error("java attacher not supported in cloud environments")
		}
	}

	g, ctx := errgroup.WithContext(s.runServerContext)

	// Ensure the libbeat output and go-elasticsearch clients do not index
	// any events to Elasticsearch before the integration is ready.
	publishReady := make(chan struct{})
	g.Go(func() error {
		defer close(publishReady)
		err := s.waitReady(ctx, kibanaClient)
		return errors.Wrap(err, "error waiting for server to be ready")
	})
	callbackUUID, err := esoutput.RegisterConnectCallback(func(*eslegclient.Connection) error {
		select {
		case <-publishReady:
			return nil
		default:
		}
		return errors.New("not ready for publishing events")
	})
	if err != nil {
		return err
	}
	defer esoutput.DeregisterConnectCallback(callbackUUID)
	newElasticsearchClient := func(cfg *elasticsearch.Config) (elasticsearch.Client, error) {
		httpTransport, err := elasticsearch.NewHTTPTransport(cfg)
		if err != nil {
			return nil, err
		}
		transport := &waitReadyRoundTripper{Transport: httpTransport, ready: publishReady}
		return elasticsearch.NewClientParams(elasticsearch.ClientParams{
			Config:    cfg,
			Transport: transport,
		})
	}

	var sourcemapFetcher sourcemap.Fetcher
	if s.config.RumConfig.Enabled && s.config.RumConfig.SourceMapping.Enabled {
		fetcher, err := newSourcemapFetcher(
			s.beat.Info, s.config.RumConfig.SourceMapping, s.fleetConfig,
			kibanaClient, newElasticsearchClient,
		)
		if err != nil {
			return err
		}
		cachingFetcher, err := sourcemap.NewCachingFetcher(
			fetcher, s.config.RumConfig.SourceMapping.Cache.Expiration,
		)
		if err != nil {
			return err
		}
		sourcemapFetcher = cachingFetcher
	}

	// Create the runServer function. We start with newBaseRunServer, and then
	// wrap depending on the configuration in order to inject behaviour.
	runServer := newBaseRunServer(listener)
	if s.tracerServer != nil {
		runServer = runServerWithTracerServer(runServer, s.tracerServer, s.tracer)
	}
	if s.wrapRunServer != nil {
		// Wrap runServer function, enabling injection of
		// behaviour into the processing/reporting pipeline.
		runServer = s.wrapRunServer(runServer)
	}
	runServer = s.wrapRunServerWithPreprocessors(runServer)

	batchProcessor := make(modelprocessor.Chained, 0, 3)
	finalBatchProcessor, closeFinalBatchProcessor, err := s.newFinalBatchProcessor(newElasticsearchClient)
	if err != nil {
		return err
	}
	batchProcessor = append(batchProcessor,
		// The server always drops non-RUM unsampled transactions. We store RUM unsampled
		// transactions as they are needed by the User Experience app, which performs
		// aggregations over dimensions that are not available in transaction metrics.
		//
		// It is important that this is done just before calling the publisher to
		// avoid affecting aggregations.
		modelprocessor.NewDropUnsampled(false /* don't drop RUM unsampled transactions*/),
		modelprocessor.DroppedSpansStatsDiscarder{},
		finalBatchProcessor,
	)

	g.Go(func() error {
		return runServer(ctx, ServerParams{
			Info:                   s.beat.Info,
			Config:                 s.config,
			Managed:                s.beat.Manager != nil && s.beat.Manager.Enabled(),
			Namespace:              s.namespace,
			Logger:                 s.logger,
			Tracer:                 s.tracer,
			BatchProcessor:         batchProcessor,
			SourcemapFetcher:       sourcemapFetcher,
			PublishReady:           publishReady,
			NewElasticsearchClient: newElasticsearchClient,
		})
	})
	result := g.Wait()
	if err := closeFinalBatchProcessor(s.backgroundContext); err != nil {
		result = multierror.Append(result, err)
	}
	return result
}

// waitReady waits until the server is ready to index events.
func (s *serverRunner) waitReady(ctx context.Context, kibanaClient kibana.Client) error {
	var preconditions []func(context.Context) error
	var esOutputClient elasticsearch.Client
	if s.elasticsearchOutputConfig != nil {
		esConfig := elasticsearch.DefaultConfig()
		err := s.elasticsearchOutputConfig.Unpack(&esConfig)
		if err != nil {
			return err
		}
		esOutputClient, err = elasticsearch.NewClient(esConfig)
		if err != nil {
			return err
		}
	}

	// libbeat and go-elasticsearch both ensure a minimum level of Basic.
	//
	// If any configured features require a higher license level, add a
	// precondition which checks this.
	if esOutputClient != nil {
		requiredLicenseLevel := licenser.Basic
		licensedFeature := ""
		if s.config.Sampling.Tail.Enabled {
			requiredLicenseLevel = licenser.Platinum
			licensedFeature = "tail-based sampling"
		}
		if requiredLicenseLevel > licenser.Basic {
			preconditions = append(preconditions, func(ctx context.Context) error {
				license, err := elasticsearch.GetLicense(ctx, esOutputClient)
				if err != nil {
					return errors.Wrap(err, "error getting Elasticsearch licensing information")
				}
				if licenser.IsExpired(license) {
					return errors.New("Elasticsearch license is expired")
				}
				if license.Type == licenser.Trial || license.Cover(requiredLicenseLevel) {
					return nil
				}
				return fmt.Errorf(
					"invalid license level %s: %s requires license level %s",
					license.Type, licensedFeature, requiredLicenseLevel,
				)
			})
		}
		preconditions = append(preconditions, func(ctx context.Context) error {
			return queryClusterUUID(ctx, esOutputClient)
		})
	}

	// When running standalone with data streams enabled, by default we will add
	// a precondition that ensures the integration is installed.
	fleetManaged := s.beat.Manager != nil && s.beat.Manager.Enabled()
	if !fleetManaged && s.config.DataStreams.WaitForIntegration {
		if kibanaClient == nil && esOutputClient == nil {
			return errors.New("cannot wait for integration without either Kibana or Elasticsearch config")
		}
		preconditions = append(preconditions, func(ctx context.Context) error {
			return checkIntegrationInstalled(ctx, kibanaClient, esOutputClient, s.logger)
		})
	}

	if len(preconditions) == 0 {
		return nil
	}
	check := func(ctx context.Context) error {
		for _, pre := range preconditions {
			if err := pre(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	return waitReady(ctx, s.config.WaitReadyInterval, s.tracer, s.logger, check)
}

// This mutex must be held when updating the libbeat monitoring registry,
// as there may be multiple servers running concurrently.
var monitoringRegistryMu sync.Mutex

// newFinalBatchProcessor returns the final model.BatchProcessor that publishes events,
// and a cleanup function which should be called on server shutdown. If the output is
// "elasticsearch", then we use modelindexer; otherwise we use the libbeat publisher.
func (s *serverRunner) newFinalBatchProcessor(
	newElasticsearchClient func(cfg *elasticsearch.Config) (elasticsearch.Client, error),
) (model.BatchProcessor, func(context.Context) error, error) {
	monitoringRegistryMu.Lock()
	defer monitoringRegistryMu.Unlock()

	if s.elasticsearchOutputConfig == nil {
		// When the publisher stops cleanly it will close its pipeline client,
		// calling the acker's Close method. We need to call Open for each new
		// publisher to ensure we wait for all clients and enqueued events to
		// be closed at shutdown time.
		s.acker.Open()
		pipeline := pipetool.WithACKer(s.pipeline, s.acker)
		publisher, err := publish.NewPublisher(pipeline, s.tracer)
		if err != nil {
			return nil, nil, err
		}
		// We only want to restore the previous libbeat registry if the output
		// has a name, otherwise, keep the libbeat registry as is. This is to
		// account for cases where the output config may be sent empty by the
		// Elastic Agent.
		if s.beat.Config != nil && s.beat.Config.Output.Name() != "" {
			monitoring.Default.Remove("libbeat")
			monitoring.Default.Add("libbeat", s.libbeatMonitoringRegistry, monitoring.Full)
		}
		return publisher, publisher.Stop, nil
	}

	var esConfig struct {
		*elasticsearch.Config `config:",inline"`
		FlushBytes            string        `config:"flush_bytes"`
		FlushInterval         time.Duration `config:"flush_interval"`
	}
	esConfig.FlushInterval = time.Second
	esConfig.Config = elasticsearch.DefaultConfig()
	if err := s.elasticsearchOutputConfig.Unpack(&esConfig); err != nil {
		return nil, nil, err
	}

	var flushBytes int
	if esConfig.FlushBytes != "" {
		b, err := humanize.ParseBytes(esConfig.FlushBytes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to parse flush_bytes")
		}
		flushBytes = int(b)
	}
	client, err := newElasticsearchClient(esConfig.Config)
	if err != nil {
		return nil, nil, err
	}
	indexer, err := modelindexer.New(client, modelindexer.Config{
		CompressionLevel: esConfig.CompressionLevel,
		FlushBytes:       flushBytes,
		FlushInterval:    esConfig.FlushInterval,
		Tracer:           s.tracer,
	})
	if err != nil {
		return nil, nil, err
	}

	// Install our own libbeat-compatible metrics callback which uses the modelindexer stats.
	// All the metrics below are required to be reported to be able to display all relevant
	// fields in the Stack Monitoring UI.
	monitoring.Default.Remove("libbeat")
	monitoring.NewFunc(monitoring.Default, "libbeat.output.write", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		v.OnKey("bytes")
		v.OnInt(indexer.Stats().BytesTotal)
	})
	outputType := monitoring.NewString(monitoring.Default.GetRegistry("libbeat.output"), "type")
	outputType.Set("elasticsearch")
	monitoring.NewFunc(monitoring.Default, "libbeat.output.bulk_requests", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		stats := indexer.Stats()
		v.OnKey("available")
		v.OnInt(stats.AvailableBulkRequests)
		v.OnKey("completed")
		v.OnInt(stats.BulkRequests)
	})
	monitoring.NewFunc(monitoring.Default, "libbeat.output.events", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		stats := indexer.Stats()
		v.OnKey("acked")
		v.OnInt(stats.Indexed)
		v.OnKey("active")
		v.OnInt(stats.Active)
		v.OnKey("batches")
		v.OnInt(stats.BulkRequests)
		v.OnKey("failed")
		v.OnInt(stats.Failed)
		v.OnKey("toomany")
		v.OnInt(stats.TooManyRequests)
		v.OnKey("total")
		v.OnInt(stats.Added)
	})
	monitoring.NewFunc(monitoring.Default, "libbeat.pipeline.events", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		v.OnKey("total")
		v.OnInt(indexer.Stats().Added)
	})
	return indexer, indexer.Close, nil
}

func (s *serverRunner) wrapRunServerWithPreprocessors(runServer RunServerFunc) RunServerFunc {
	processors := []model.BatchProcessor{
		modelprocessor.SetHostHostname{},
		modelprocessor.SetServiceNodeName{},
		modelprocessor.SetMetricsetName{},
		modelprocessor.SetGroupingKey{},
		modelprocessor.SetErrorMessage{},
		newObserverBatchProcessor(s.beat.Info),
		model.ProcessBatchFunc(ecsVersionBatchProcessor),
		modelprocessor.NewEventCounter(monitoring.Default.GetRegistry("apm-server")),
		&modelprocessor.SetDataStream{Namespace: s.namespace},
		modelprocessor.SetUnknownSpanType{},
	}
	if s.config.DefaultServiceEnvironment != "" {
		processors = append(processors, &modelprocessor.SetDefaultServiceEnvironment{
			DefaultServiceEnvironment: s.config.DefaultServiceEnvironment,
		})
	}
	return WrapRunServerWithProcessors(runServer, processors...)
}

func hasElasticsearchOutput(b *beat.Beat) bool {
	return b.Config != nil && b.Config.Output.Name() == "elasticsearch"
}

func initTracing(b *beat.Beat, cfg *config.Config, logger *logp.Logger) (*apm.Tracer, *tracerServer, error) {
	tracer := b.Instrumentation.Tracer()
	listener := b.Instrumentation.Listener()

	var tracerServer *tracerServer
	if listener != nil {
		var err error
		tracerServer, err = newTracerServer(listener, logger)
		if err != nil {
			return nil, nil, err
		}
	}
	return tracer, tracerServer, nil
}

// Stop stops the beater gracefully.
func (bt *beater) Stop() {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()
	if bt.stopped || bt.stopServer == nil {
		return
	}
	bt.logger.Infof(
		"stopping apm-server... waiting maximum of %v seconds for queues to drain",
		bt.config.ShutdownTimeout.Seconds(),
	)
	bt.outputConfigReloader.cancel()
	bt.stopServer()
	bt.stopped = true
}

// runServerWithTracerServer wraps runServer such that it also runs
// tracerServer, stopping it and the tracer when the server shuts down.
func runServerWithTracerServer(runServer RunServerFunc, tracerServer *tracerServer, tracer *apm.Tracer) RunServerFunc {
	return func(ctx context.Context, args ServerParams) error {
		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() error {
			return tracerServer.serve(ctx, args.BatchProcessor)
		})
		g.Go(func() error {
			return runServer(ctx, args)
		})
		return g.Wait()
	}
}

func newSourcemapFetcher(
	beatInfo beat.Info,
	cfg config.SourceMapping,
	fleetCfg *config.Fleet,
	kibanaClient kibana.Client,
	newElasticsearchClient func(*elasticsearch.Config) (elasticsearch.Client, error),
) (sourcemap.Fetcher, error) {
	// When running under Fleet we only fetch via Fleet Server.
	if fleetCfg != nil {
		var tlsConfig *tlscommon.TLSConfig
		var err error
		if fleetCfg.TLS.IsEnabled() {
			if tlsConfig, err = tlscommon.LoadTLSConfig(fleetCfg.TLS); err != nil {
				return nil, err
			}
		}

		timeout := 30 * time.Second
		dialer := transport.NetDialer(timeout)
		tlsDialer := transport.TLSDialer(dialer, tlsConfig, timeout)

		client := *http.DefaultClient
		client.Transport = apmhttp.WrapRoundTripper(&http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			Dial:            dialer.Dial,
			DialTLS:         tlsDialer.Dial,
			TLSClientConfig: tlsConfig.ToConfig(),
		})

		return sourcemap.NewFleetFetcher(&client, fleetCfg, cfg.Metadata)
	}

	// For standalone, we query both Kibana and Elasticsearch for backwards compatibility.
	var chained sourcemap.ChainedFetcher
	if kibanaClient != nil {
		chained = append(chained, sourcemap.NewKibanaFetcher(kibanaClient))
	}
	esClient, err := newElasticsearchClient(cfg.ESConfig)
	if err != nil {
		return nil, err
	}
	index := strings.ReplaceAll(cfg.IndexPattern, "%{[observer.version]}", beatInfo.Version)
	esFetcher := sourcemap.NewElasticsearchFetcher(esClient, index)
	chained = append(chained, esFetcher)
	return chained, nil
}

// WrapRunServerWithProcessors wraps runServer such that it wraps args.Reporter
// with a function that event batches are first passed through the given processors
// in order.
func WrapRunServerWithProcessors(runServer RunServerFunc, processors ...model.BatchProcessor) RunServerFunc {
	if len(processors) == 0 {
		return runServer
	}
	return func(ctx context.Context, args ServerParams) error {
		processors := append(processors, args.BatchProcessor)
		args.BatchProcessor = modelprocessor.Chained(processors)
		return runServer(ctx, args)
	}
}

// chanReloader implements libbeat/common/reload.Reloadable, converting
// Reload calls into requests send to a channel consumed by serve.
type chanReloader struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan reloadRequest
}

func newChanReloader() *chanReloader {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan reloadRequest)
	return &chanReloader{ctx, cancel, ch}
}

type reloadRequest struct {
	cfg    *reload.ConfigWithMeta
	result chan<- error
}

// Reload sends a reload request to r.ch, which is consumed by another
// goroutine running r.serve. Reload blocks until serve has handled the
// reload request, or until the reloader's context has been cancelled.
func (r *chanReloader) Reload(cfg *reload.ConfigWithMeta) error {
	result := make(chan error, 1)
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case r.ch <- reloadRequest{cfg: cfg, result: result}:
	}
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case err := <-result:
		return err
	}
}

// serve handles reload requests enqueued by Reload, returning when either
// ctx or r.ctx are cancelled.
func (r *chanReloader) serve(ctx context.Context, reloader reload.Reloadable) error {
	for {
		select {
		case <-r.ctx.Done():
			return r.ctx.Err()
		case <-ctx.Done():
			return ctx.Err()
		case req := <-r.ch:
			err := reloader.Reload(req.cfg)
			select {
			case <-r.ctx.Done():
				return r.ctx.Err()
			case <-ctx.Done():
				return ctx.Err()
			case req.result <- err:
			}
		}
	}
}

// TODO: This is copying behavior from libbeat:
// https://github.com/elastic/beats/blob/b9ced47dba8bb55faa3b2b834fd6529d3c4d0919/libbeat/cmd/instance/beat.go#L927-L950
// Remove this when cluster_uuid no longer needs to be queried from ES.
func queryClusterUUID(ctx context.Context, esClient elasticsearch.Client) error {
	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	outputES := "outputs.elasticsearch"
	// Running under elastic-agent, the callback linked above is not
	// registered until later, meaning we need to check and instantiate the
	// registries if they don't exist.
	elasticsearchRegistry := stateRegistry.GetRegistry(outputES)
	if elasticsearchRegistry == nil {
		elasticsearchRegistry = stateRegistry.NewRegistry(outputES)
	}

	var (
		s  *monitoring.String
		ok bool
	)

	clusterUUID := "cluster_uuid"
	clusterUUIDRegVar := elasticsearchRegistry.Get(clusterUUID)
	if clusterUUIDRegVar != nil {
		s, ok = clusterUUIDRegVar.(*monitoring.String)
		if !ok {
			return fmt.Errorf("couldn't cast to String")
		}
	} else {
		s = monitoring.NewString(elasticsearchRegistry, clusterUUID)
	}

	var response struct {
		ClusterUUID string `json:"cluster_uuid"`
	}

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		return err
	}
	resp, err := esClient.Perform(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return fmt.Errorf("error querying cluster_uuid: status_code=%d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return err
	}

	s.Set(response.ClusterUUID)
	return nil
}
