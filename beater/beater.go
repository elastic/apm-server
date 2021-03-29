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
	"net"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/kibana"

	"github.com/pkg/errors"
	"go.elastic.co/apm"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/instrumentation"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/management"
	esoutput "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/ingest/pipeline"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/sampling"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/apm-server/transform"
)

var (
	errSetupDashboardRemoved = errors.New("setting 'setup.dashboards' has been removed")
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
		if err := checkConfig(logger); err != nil {
			return nil, err
		}
		bt := &beater{
			rawConfig:     ucfg,
			stopped:       false,
			logger:        logger,
			wrapRunServer: args.WrapRunServer,
			waitPublished: newWaitPublishedAcker(),
		}

		var err error
		bt.config, err = config.NewConfig(bt.rawConfig, elasticsearchOutputConfig(b))
		if err != nil {
			return nil, err
		}
		if err := recordRootConfig(b.Info, bt.rawConfig); err != nil {
			bt.logger.Errorf("Error recording telemetry data", err)
		}

		if bt.config.Pprof.IsEnabled() {
			// Profiling rates should be set once, early on in the program.
			runtime.SetBlockProfileRate(bt.config.Pprof.BlockProfileRate)
			runtime.SetMutexProfileFraction(bt.config.Pprof.MutexProfileRate)
			if bt.config.Pprof.MemProfileRate > 0 {
				runtime.MemProfileRate = bt.config.Pprof.MemProfileRate
			}
		}

		if !bt.config.DataStreams.Enabled {
			if b.Manager != nil && b.Manager.Enabled() {
				return nil, errors.New("data streams must be enabled when the server is managed")
			}
		}

		if err := bt.registerPipelineCallback(b); err != nil {
			return nil, err
		}

		return bt, nil
	}
}

type beater struct {
	rawConfig     *common.Config
	config        *config.Config
	logger        *logp.Logger
	wrapRunServer func(RunServerFunc) RunServerFunc
	waitPublished *waitPublishedAcker

	mutex      sync.Mutex // guards stopServer and stopped
	stopServer func()
	stopped    bool
}

// Run runs the APM Server, blocking until the beater's Stop method is called,
// or a fatal error occurs.
func (bt *beater) Run(b *beat.Beat) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done, err := bt.start(ctx, cancel, b)
	if err != nil {
		return err
	}
	<-done
	bt.waitPublished.Wait(ctx)
	return nil
}

func (bt *beater) start(ctx context.Context, cancelContext context.CancelFunc, b *beat.Beat) (<-chan struct{}, error) {
	done := make(chan struct{})
	bt.mutex.Lock()
	defer bt.mutex.Unlock()
	if bt.stopped {
		close(done)
		return done, nil
	}

	tracer, tracerServer, err := initTracing(b, bt.config, bt.logger)
	if err != nil {
		return nil, err
	}
	closeTracer := func() error { return nil }
	if tracer != nil {
		closeTracer = func() error {
			tracer.Close()
			if tracerServer != nil {
				return tracerServer.Close()
			}
			return nil
		}
	}

	sharedArgs := sharedServerRunnerParams{
		Beat:          b,
		WrapRunServer: bt.wrapRunServer,
		Logger:        bt.logger,
		Tracer:        tracer,
		TracerServer:  tracerServer,
		Acker:         bt.waitPublished,
	}

	if b.Manager != nil && b.Manager.Enabled() {
		// Management enabled, register reloadable inputs.
		creator := &serverCreator{context: ctx, args: sharedArgs}
		inputs := cfgfile.NewRunnerList(management.DebugK, creator, b.Publisher)
		bt.stopServer = func() {
			defer close(done)
			defer closeTracer()
			if bt.config.ShutdownTimeout > 0 {
				time.AfterFunc(bt.config.ShutdownTimeout, cancelContext)
			}
			inputs.Stop()
		}
		reload.Register.MustRegisterList("inputs", inputs)

	} else {
		// Management disabled, use statically defined config.
		s, err := newServerRunner(ctx, serverRunnerParams{
			sharedServerRunnerParams: sharedArgs,
			Pipeline:                 b.Publisher,
			Namespace:                "default",
			RawConfig:                bt.rawConfig,
		})
		if err != nil {
			return nil, err
		}
		bt.stopServer = func() {
			if bt.config.ShutdownTimeout > 0 {
				time.AfterFunc(bt.config.ShutdownTimeout, cancelContext)
			}
			s.Stop()
		}
		s.Start()
		go func() {
			defer close(done)
			defer closeTracer()
			s.Wait()
		}()
	}
	return done, nil
}

type serverCreator struct {
	context context.Context
	args    sharedServerRunnerParams
}

func (s *serverCreator) CheckConfig(cfg *common.Config) error {
	_, err := config.NewIntegrationConfig(cfg)
	return err
}

func (s *serverCreator) Create(p beat.PipelineConnector, rawConfig *common.Config) (cfgfile.Runner, error) {
	integrationConfig, err := config.NewIntegrationConfig(rawConfig)
	if err != nil {
		return nil, err
	}
	var namespace string
	if integrationConfig.DataStream != nil {
		namespace = integrationConfig.DataStream.Namespace
	}
	apmServerCommonConfig := integrationConfig.APMServer
	apmServerCommonConfig.Merge(common.MustNewConfigFrom(`{"data_streams.enabled": true}`))
	return newServerRunner(s.context, serverRunnerParams{
		sharedServerRunnerParams: s.args,
		Namespace:                namespace,
		Pipeline:                 p,
		KibanaConfig:             &integrationConfig.Fleet.Kibana,
		RawConfig:                apmServerCommonConfig,
	})
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

	stopOnce sync.Once
	wg       sync.WaitGroup

	pipeline      beat.PipelineConnector
	acker         *waitPublishedAcker
	namespace     string
	config        *config.Config
	beat          *beat.Beat
	logger        *logp.Logger
	tracer        *apm.Tracer
	tracerServer  *tracerServer
	wrapRunServer func(RunServerFunc) RunServerFunc
}

type serverRunnerParams struct {
	sharedServerRunnerParams

	Namespace    string
	Pipeline     beat.PipelineConnector
	KibanaConfig *kibana.ClientConfig
	RawConfig    *common.Config
}

type sharedServerRunnerParams struct {
	Beat          *beat.Beat
	WrapRunServer func(RunServerFunc) RunServerFunc
	Logger        *logp.Logger
	Tracer        *apm.Tracer
	TracerServer  *tracerServer
	Acker         *waitPublishedAcker
}

func newServerRunner(ctx context.Context, args serverRunnerParams) (*serverRunner, error) {
	cfg, err := config.NewConfig(args.RawConfig, elasticsearchOutputConfig(args.Beat))
	if err != nil {
		return nil, err
	}

	if cfg.DataStreams.Enabled && args.KibanaConfig != nil {
		cfg.Kibana.ClientConfig = *args.KibanaConfig
	}

	runServerContext, cancel := context.WithCancel(ctx)
	return &serverRunner{
		backgroundContext:      ctx,
		runServerContext:       runServerContext,
		cancelRunServerContext: cancel,

		config:        cfg,
		acker:         args.Acker,
		pipeline:      args.Pipeline,
		namespace:     args.Namespace,
		beat:          args.Beat,
		logger:        args.Logger,
		tracer:        args.Tracer,
		tracerServer:  args.TracerServer,
		wrapRunServer: args.WrapRunServer,
	}, nil
}

func (s *serverRunner) String() string {
	return "APMServer"
}

// Stop stops the server.
func (s *serverRunner) Stop() {
	s.stopOnce.Do(s.cancelRunServerContext)
	s.Wait()
}

// Wait waits for the server to stop.
func (s *serverRunner) Wait() {
	s.wg.Wait()
}

// Start starts the server.
func (s *serverRunner) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run()
	}()
}

func (s *serverRunner) run() error {
	// Send config to telemetry.
	recordAPMServerConfig(s.config)

	transformConfig, err := newTransformConfig(s.beat.Info, s.config)
	if err != nil {
		return err
	}
	publisherConfig := &publish.PublisherConfig{
		Info:            s.beat.Info,
		Pipeline:        s.config.Pipeline,
		Namespace:       s.namespace,
		TransformConfig: transformConfig,
	}

	// When the publisher stops cleanly it will close its pipeline client,
	// calling the acker's Close method. We need to call Open for each new
	// publisher to ensure we wait for all clients and enqueued events to
	// be closed at shutdown time.
	s.acker.Open()
	pipeline := pipetool.WithACKer(s.pipeline, s.acker)

	publisher, err := publish.NewPublisher(pipeline, s.tracer, publisherConfig)
	if err != nil {
		return err
	}
	defer publisher.Stop(s.backgroundContext)

	// Create the runServer function. We start with newBaseRunServer, and then
	// wrap depending on the configuration in order to inject behaviour.
	reporter := publisher.Send
	runServer := newBaseRunServer(reporter)
	if s.tracerServer != nil {
		runServer = runServerWithTracerServer(runServer, s.tracerServer, s.tracer)
	}
	if s.wrapRunServer != nil {
		// Wrap runServer function, enabling injection of
		// behaviour into the processing/reporting pipeline.
		runServer = s.wrapRunServer(runServer)
	}
	runServer = s.wrapRunServerWithPreprocessors(runServer)

	var batchProcessor model.BatchProcessor = &reporterBatchProcessor{reporter}
	if !s.config.Sampling.KeepUnsampled {
		// The server has been configured to discard unsampled
		// transactions. Make sure this is done just before calling
		// the publisher to avoid affecting aggregations.
		batchProcessor = modelprocessor.Chained{
			sampling.NewDiscardUnsampledBatchProcessor(), batchProcessor,
		}
	}

	if err := runServer(s.runServerContext, ServerParams{
		Info:           s.beat.Info,
		Config:         s.config,
		Managed:        s.beat.Manager != nil && s.beat.Manager.Enabled(),
		Namespace:      s.namespace,
		Logger:         s.logger,
		Tracer:         s.tracer,
		BatchProcessor: batchProcessor,
	}); err != nil {
		return err
	}
	return publisher.Stop(s.backgroundContext)
}

func (s *serverRunner) wrapRunServerWithPreprocessors(runServer RunServerFunc) RunServerFunc {
	processors := []model.BatchProcessor{
		modelprocessor.SetSystemHostname{},
		modelprocessor.SetServiceNodeName{},
		// Set metricset.name for well-known agent metrics.
		modelprocessor.SetMetricsetName{},
	}
	if s.config.DefaultServiceEnvironment != "" {
		processors = append(processors, &modelprocessor.SetDefaultServiceEnvironment{
			DefaultServiceEnvironment: s.config.DefaultServiceEnvironment,
		})
	}
	return WrapRunServerWithProcessors(runServer, processors...)
}

// checkConfig verifies the global configuration doesn't use unsupported settings
//
// TODO(axw) remove this, nobody expects dashboard setup from apm-server.
func checkConfig(logger *logp.Logger) error {
	cfg, err := cfgfile.Load("", nil)
	if err != nil {
		// responsibility for failing to load configuration lies elsewhere
		// this is not reachable after going through normal beat creation
		return nil
	}

	var s struct {
		Dashboards *common.Config `config:"setup.dashboards"`
	}
	if err := cfg.Unpack(&s); err != nil {
		return err
	}
	if s.Dashboards != nil {
		if s.Dashboards.Enabled() {
			return errSetupDashboardRemoved
		}
		logger.Warn(errSetupDashboardRemoved)
	}
	return nil
}

// elasticsearchOutputConfig returns nil if the output is not elasticsearch
func elasticsearchOutputConfig(b *beat.Beat) *common.Config {
	if hasElasticsearchOutput(b) {
		return b.Config.Output.Config()
	}
	return nil
}

func hasElasticsearchOutput(b *beat.Beat) bool {
	return b.Config != nil && b.Config.Output.Name() == "elasticsearch"
}

// registerPipelineCallback registers an Elasticsearch connection callback
// that ensures the configured pipeline is installed, if configured to do
// so. If data streams are enabled, then pipeline registration is always
// disabled and `setup --pipelines` will return an error.
func (bt *beater) registerPipelineCallback(b *beat.Beat) error {
	if !hasElasticsearchOutput(b) {
		bt.logger.Info("Output is not Elasticsearch: pipeline registration disabled")
		return nil
	}

	if bt.config.DataStreams.Enabled {
		bt.logger.Info("Data streams enabled: pipeline registration disabled")
		b.OverwritePipelinesCallback = func(esConfig *common.Config) error {
			return errors.New("index pipeline setup must be performed externally when using data streams, by installing the 'apm' integration package")
		}
		return nil
	}

	if !bt.config.Register.Ingest.Pipeline.IsEnabled() {
		bt.logger.Info("Pipeline registration disabled")
		return nil
	}

	bt.logger.Info("Registering pipeline callback")
	overwrite := bt.config.Register.Ingest.Pipeline.ShouldOverwrite()
	path := bt.config.Register.Ingest.Pipeline.Path

	// ensure setup cmd is working properly
	b.OverwritePipelinesCallback = func(esConfig *common.Config) error {
		conn, err := eslegclient.NewConnectedClient(esConfig)
		if err != nil {
			return err
		}
		return pipeline.RegisterPipelines(conn, overwrite, path)
	}
	// ensure pipelines are registered when new ES connection is established.
	_, err := esoutput.RegisterConnectCallback(func(conn *eslegclient.Connection) error {
		return pipeline.RegisterPipelines(conn, overwrite, path)
	})
	return err
}

func initTracing(b *beat.Beat, cfg *config.Config, logger *logp.Logger) (*apm.Tracer, *tracerServer, error) {
	tracer := b.Instrumentation.Tracer()
	listener := b.Instrumentation.Listener()

	if !tracer.Active() && cfg != nil {
		var err error
		tracer, listener, err = initLegacyTracer(b.Info, cfg)
		if err != nil {
			return nil, nil, err
		}
	}

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

// initLegacyTracer exists for backwards compatibility and it should be removed in 8.0
// it does not instrument the beat output
func initLegacyTracer(info beat.Info, cfg *config.Config) (*apm.Tracer, net.Listener, error) {
	selfInstrumentation := cfg.SelfInstrumentation
	if selfInstrumentation == nil || !selfInstrumentation.IsEnabled() {
		return apm.DefaultTracer, nil, nil
	}
	conf, err := common.NewConfigFrom(cfg.SelfInstrumentation)
	if err != nil {
		return nil, nil, err
	}
	// this is needed because `hosts` strings are unpacked as URL's, so we need to covert them back to strings
	// to not break ucfg - this code path is exercised in TestExternalTracing* system tests
	for idx, h := range selfInstrumentation.Hosts {
		err := conf.SetString("hosts", idx, h.String())
		if err != nil {
			return nil, nil, err
		}
	}
	parent := common.NewConfig()
	err = parent.SetChild("instrumentation", -1, conf)
	if err != nil {
		return nil, nil, err
	}

	instr, err := instrumentation.New(parent, info.Beat, info.Version)
	if err != nil {
		return nil, nil, err
	}
	return instr.Tracer(), instr.Listener(), nil
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

func newTransformConfig(beatInfo beat.Info, cfg *config.Config) (*transform.Config, error) {
	transformConfig := &transform.Config{
		DataStreams: cfg.DataStreams.Enabled,
		RUM: transform.RUMConfig{
			LibraryPattern:      regexp.MustCompile(cfg.RumConfig.LibraryPattern),
			ExcludeFromGrouping: regexp.MustCompile(cfg.RumConfig.ExcludeFromGrouping),
			MaxLineLength:       cfg.RumConfig.SourceMapping.MaxLineLength,
		},
	}

	if cfg.RumConfig.IsEnabled() && cfg.RumConfig.SourceMapping.IsEnabled() && cfg.RumConfig.SourceMapping.ESConfig != nil {
		store, err := newSourcemapStore(beatInfo, cfg.RumConfig.SourceMapping)
		if err != nil {
			return nil, err
		}
		transformConfig.RUM.SourcemapStore = store
	}

	return transformConfig, nil
}

func newSourcemapStore(beatInfo beat.Info, cfg *config.SourceMapping) (*sourcemap.Store, error) {
	esClient, err := elasticsearch.NewClient(cfg.ESConfig)
	if err != nil {
		return nil, err
	}
	index := strings.ReplaceAll(cfg.IndexPattern, "%{[observer.version]}", beatInfo.Version)
	return sourcemap.NewStore(esClient, index, cfg.Cache.Expiration)
}

// WrapRunServerWithProcessors wraps runServer such that it wraps args.Reporter
// with a function that event batches are first passed through the given processors
// in order.
func WrapRunServerWithProcessors(runServer RunServerFunc, processors ...model.BatchProcessor) RunServerFunc {
	if len(processors) == 0 {
		return runServer
	}
	return func(ctx context.Context, args ServerParams) error {
		processors = append(processors, args.BatchProcessor)
		args.BatchProcessor = modelprocessor.Chained(processors)
		return runServer(ctx, args)
	}
}

type disablePublisherTracingKey struct{}

type reporterBatchProcessor struct {
	reporter publish.Reporter
}

func (p *reporterBatchProcessor) ProcessBatch(ctx context.Context, batch *model.Batch) error {
	disableTracing, _ := ctx.Value(disablePublisherTracingKey{}).(bool)
	return p.reporter(ctx, publish.PendingReq{Transformable: batch, Trace: !disableTracing})
}
