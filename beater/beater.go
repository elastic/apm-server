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
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/v7/libbeat/common/fleetmode"
	"github.com/elastic/beats/v7/libbeat/common/transport"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/go-ucfg"

	"github.com/pkg/errors"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/instrumentation"
	"github.com/elastic/beats/v7/libbeat/logp"
	esoutput "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/ingest/pipeline"
	kibana_client "github.com/elastic/apm-server/kibana"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/sampling"
	"github.com/elastic/apm-server/sourcemap"
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

		if bt.config.Pprof.Enabled {
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
		} else if bt.config.DataStreams.Enabled && !fleetmode.Enabled() {
			// not supported only available for development purposes
			bt.logger.Errorf("Started apm-server with data streams enabled but no active fleet management mode was specified")
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

	reloader := reloader{
		runServerContext: ctx,
		args:             sharedArgs,
	}
	bt.stopServer = func() {
		defer close(done)
		defer closeTracer()
		if bt.config.ShutdownTimeout > 0 {
			time.AfterFunc(bt.config.ShutdownTimeout, cancelContext)
		}
		reloader.stop()
	}
	if b.Manager != nil && b.Manager.Enabled() {
		reload.Register.MustRegisterList("inputs", &reloader)
	} else {
		// Management disabled, use statically defined config.
		if err := reloader.reload(bt.rawConfig, "default", nil); err != nil {
			return nil, err
		}
	}
	return done, nil
}

type reloader struct {
	runServerContext context.Context
	args             sharedServerRunnerParams
	// The json marshaled bytes of config.Config, with all the dynamic
	// options zeroed out.
	staticConfig []byte

	mu     sync.Mutex
	runner *serverRunner
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
	apmServerCommonConfig := integrationConfig.APMServer
	apmServerCommonConfig.Merge(common.MustNewConfigFrom(`{"data_streams.enabled": true}`))
	return r.reload(apmServerCommonConfig, namespace, &integrationConfig.Fleet)
}

func (r *reloader) reload(rawConfig *common.Config, namespace string, fleetConfig *config.Fleet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg, err := config.NewConfig(rawConfig, elasticsearchOutputConfig(r.args.Beat))
	if err != nil {
		return err
	}
	dynamicCfg, shouldRestart, err := r.splitCfg(cfg)
	if err != nil {
		return err
	}

	if shouldRestart {
		if r.runner != nil {
			r.runner.cancelRunServerContext()
			<-r.runner.done
			r.runner = nil
		}
		runner, err := newServerRunner(r.runServerContext, serverRunnerParams{
			sharedServerRunnerParams: r.args,
			Namespace:                namespace,
			RawConfig:                rawConfig,
			FleetConfig:              fleetConfig,
		})
		if err != nil {
			return err
		}
		r.runner = runner
		go r.runner.run()
		return nil
	}
	// Update current runner.
	// TODO: This should actually update the runner.
	return r.runner.updateDynamicConfig(dynamicCfg)
}

func (r *reloader) splitCfg(cfg *config.Config) (dynamicConfig, bool, error) {
	// Create the dynamic config
	dcfg := dynamicConfig{
		maxHeaderSize:             cfg.MaxHeaderSize,
		idleTimeout:               cfg.IdleTimeout,
		readTimeout:               cfg.ReadTimeout,
		writeTimeout:              cfg.WriteTimeout,
		maxEventSize:              cfg.MaxEventSize,
		shutdownTimeout:           cfg.ShutdownTimeout,
		responseHeaders:           cfg.ResponseHeaders,
		augmentEnabled:            cfg.AugmentEnabled,
		rateLimit:                 cfg.AgentAuth.Anonymous.RateLimit,
		expvar:                    cfg.Expvar,
		rumConfig:                 cfg.RumConfig,
		apiKeyLimit:               cfg.AgentAuth.APIKey.LimitPerMin,
		defaultServiceEnvironment: cfg.DefaultServiceEnvironment,
		agentConfigs:              cfg.AgentConfigs,
	}
	// Zero out dynamic values in the main config
	cfg.MaxHeaderSize = 0
	cfg.IdleTimeout = 0
	cfg.ReadTimeout = 0
	cfg.WriteTimeout = 0
	cfg.MaxEventSize = 0
	cfg.ShutdownTimeout = 0
	cfg.ResponseHeaders = nil
	cfg.AugmentEnabled = false
	cfg.AgentAuth.Anonymous.RateLimit = config.RateLimit{}
	cfg.Expvar = config.ExpvarConfig{}
	cfg.RumConfig = config.RumConfig{}
	cfg.AgentAuth.APIKey.LimitPerMin = 0
	cfg.DefaultServiceEnvironment = ""
	cfg.AgentConfigs = nil
	// Does our static config match the previous static config? If not,
	// then we should restart.
	m, err := json.Marshal(cfg)
	if err != nil {
		return dynamicConfig{}, false, err
	}

	shouldRestart := !reflect.DeepEqual(m, r.staticConfig)
	// Set the static config on reloader for the next comparison
	r.staticConfig = m

	return dcfg, shouldRestart, nil
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

	pipeline      beat.PipelineConnector
	acker         *waitPublishedAcker
	namespace     string
	config        *config.Config
	rawConfig     *common.Config
	fleetConfig   *config.Fleet
	beat          *beat.Beat
	logger        *logp.Logger
	tracer        *apm.Tracer
	tracerServer  *tracerServer
	wrapRunServer func(RunServerFunc) RunServerFunc
}

type serverRunnerParams struct {
	sharedServerRunnerParams

	Namespace   string
	RawConfig   *common.Config
	FleetConfig *config.Fleet
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

	runServerContext, cancel := context.WithCancel(ctx)
	return &serverRunner{
		backgroundContext:      ctx,
		runServerContext:       runServerContext,
		cancelRunServerContext: cancel,
		done:                   make(chan struct{}),

		config:        cfg,
		rawConfig:     args.RawConfig,
		fleetConfig:   args.FleetConfig,
		acker:         args.Acker,
		pipeline:      args.Beat.Publisher,
		namespace:     args.Namespace,
		beat:          args.Beat,
		logger:        args.Logger,
		tracer:        args.Tracer,
		tracerServer:  args.TracerServer,
		wrapRunServer: args.WrapRunServer,
	}, nil
}

func (s *serverRunner) run() error {
	defer close(s.done)

	// Send config to telemetry.
	recordAPMServerConfig(s.config)
	publisherConfig := &publish.PublisherConfig{
		Info:      s.beat.Info,
		Pipeline:  s.config.Pipeline,
		Namespace: s.namespace,
	}

	cfg := ucfg.Config(*s.rawConfig)
	parentCfg := cfg.Parent()
	// Check for an environment variable set when running in a cloud environment
	if eac := os.Getenv("ELASTIC_AGENT_CLOUD"); eac != "" && s.config.Kibana.Enabled {
		// Don't block server startup sending the config.
		go func() {
			c := kibana_client.NewConnectingClient(&s.config.Kibana)
			if err := kibana_client.SendConfig(s.runServerContext, c, parentCfg); err != nil {
				s.logger.Infof("failed to upload config to kibana: %v", err)
			}
		}()
	}

	var sourcemapStore *sourcemap.Store
	if s.config.RumConfig.Enabled && s.config.RumConfig.SourceMapping.Enabled {
		store, err := newSourcemapStore(s.beat.Info, s.config.RumConfig.SourceMapping, s.fleetConfig)
		if err != nil {
			return err
		}
		sourcemapStore = store
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
		SourcemapStore: sourcemapStore,
	}); err != nil {
		return err
	}
	return publisher.Stop(s.backgroundContext)
}

type dynamicConfig struct {
	maxHeaderSize   int
	idleTimeout     time.Duration
	readTimeout     time.Duration
	writeTimeout    time.Duration
	maxEventSize    int
	shutdownTimeout time.Duration

	responseHeaders map[string][]string
	augmentEnabled  bool

	rateLimit config.RateLimit
	expvar    config.ExpvarConfig
	rumConfig config.RumConfig

	// Per minute limit on number of unique API keys used for auth between
	// agents and apm-server
	apiKeyLimit int

	defaultServiceEnvironment string

	agentConfigs []config.AgentConfig
}

func (s *serverRunner) updateDynamicConfig(cfg dynamicConfig) error {
	return nil
}

func (s *serverRunner) wrapRunServerWithPreprocessors(runServer RunServerFunc) RunServerFunc {
	processors := []model.BatchProcessor{
		modelprocessor.SetHostHostname{},
		modelprocessor.SetServiceNodeName{},
		modelprocessor.SetMetricsetName{},
		modelprocessor.SetGroupingKey{},
	}
	if s.config.DefaultServiceEnvironment != "" {
		processors = append(processors, &modelprocessor.SetDefaultServiceEnvironment{
			DefaultServiceEnvironment: s.config.DefaultServiceEnvironment,
		})
	}
	if s.config.DataStreams.Enabled {
		processors = append(processors, &modelprocessor.SetDataStream{
			Namespace: s.namespace,
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

	if !bt.config.Register.Ingest.Pipeline.Enabled {
		bt.logger.Info("Pipeline registration disabled")
		return nil
	}

	bt.logger.Info("Registering pipeline callback")
	overwrite := bt.config.Register.Ingest.Pipeline.Overwrite
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
	if !selfInstrumentation.Enabled {
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

func newSourcemapStore(beatInfo beat.Info, cfg config.SourceMapping, fleetCfg *config.Fleet) (*sourcemap.Store, error) {
	if fleetCfg != nil {
		var (
			c  = *http.DefaultClient
			rt = http.DefaultTransport
		)
		var tlsConfig *tlscommon.TLSConfig
		var err error
		if fleetCfg.TLS.IsEnabled() {
			if tlsConfig, err = tlscommon.LoadTLSConfig(fleetCfg.TLS); err != nil {
				return nil, err
			}
		}

		// Default for es is 90s :shrug:
		timeout := 30 * time.Second
		dialer := transport.NetDialer(timeout)
		tlsDialer := transport.TLSDialer(dialer, tlsConfig, timeout)

		rt = &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			Dial:            dialer.Dial,
			DialTLS:         tlsDialer.Dial,
			TLSClientConfig: tlsConfig.ToConfig(),
		}

		c.Transport = apmhttp.WrapRoundTripper(rt)
		return sourcemap.NewFleetStore(&c, fleetCfg, cfg.Metadata, cfg.Cache.Expiration)
	}
	c, err := elasticsearch.NewClient(cfg.ESConfig)
	if err != nil {
		return nil, err
	}
	index := strings.ReplaceAll(cfg.IndexPattern, "%{[observer.version]}", beatInfo.Version)
	return sourcemap.NewElasticsearchStore(c, index, cfg.Cache.Expiration)
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

type disablePublisherTracingKey struct{}

type reporterBatchProcessor struct {
	reporter publish.Reporter
}

func (p *reporterBatchProcessor) ProcessBatch(ctx context.Context, batch *model.Batch) error {
	disableTracing, _ := ctx.Value(disablePublisherTracingKey{}).(bool)
	return p.reporter(ctx, publish.PendingReq{Transformable: batch, Trace: !disableTracing})
}
