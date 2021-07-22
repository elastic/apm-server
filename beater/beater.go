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
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
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
	"github.com/elastic/beats/v7/libbeat/management"
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
	runner  *serverRunner
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
	params := serverRunnerParams{
		sharedServerRunnerParams: s.args,
		Namespace:                namespace,
		Pipeline:                 p,
		RawConfig:                apmServerCommonConfig,
		FleetConfig:              &integrationConfig.Fleet,
	}
	if s.runner == nil || s.runner.status == runnerDone {
		// If we don't have a runner, create it.
		// If the runner is done, create a new one.
		s.runner, err = newServerRunner(s.context, params)
		return s.runner, err
	} else {
		// If we've already created a runner, just reconfigure it.
		return s.runner, s.runner.configure(params)
	}
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

	restartc chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	mu              sync.Mutex
	sourcemapStore  *sourcemap.Store
	status          runnerStatus
	publisher       *publish.Publisher
	pipeline        beat.PipelineConnector
	batchProcessor  model.BatchProcessor
	acker           *waitPublishedAcker
	namespace       string
	config          *config.Config
	rawConfig       *common.Config
	fleetConfig     *config.Fleet
	beat            *beat.Beat
	logger          *logp.Logger
	tracer          *apm.Tracer
	tracerServer    *tracerServer
	wrapRunServer   func(RunServerFunc) RunServerFunc
	configureParams func(*ServerParams)

	server *server
	// delete me
	params serverRunnerParams
}

type runnerStatus int

const (
	runnerCreated runnerStatus = iota
	runnerRunning
	runnerDone
)

type serverRunnerParams struct {
	sharedServerRunnerParams

	Namespace   string
	Pipeline    beat.PipelineConnector
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
	runServerContext, cancel := context.WithCancel(ctx)
	config, err := config.NewConfig(args.RawConfig, elasticsearchOutputConfig(args.Beat))
	if err != nil {
		return nil, err
	}
	s := &serverRunner{
		backgroundContext:      ctx,
		runServerContext:       runServerContext,
		cancelRunServerContext: cancel,
		config:                 config,
		rawConfig:              args.RawConfig,
		fleetConfig:            args.FleetConfig,
		acker:                  args.Acker,
		pipeline:               args.Pipeline,
		namespace:              args.Namespace,
		beat:                   args.Beat,
		logger:                 args.Logger,
		tracer:                 args.Tracer,
		tracerServer:           args.TracerServer,
		wrapRunServer:          args.WrapRunServer,
		restartc:               make(chan struct{}),
		status:                 runnerCreated,
		// This is just for testing right now.
		params: args,
	}
	if err := s.configure(args); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *serverRunner) configure(args serverRunnerParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := config.NewConfig(args.RawConfig, elasticsearchOutputConfig(args.Beat))
	if err != nil {
		return err
	}

	// once the server has been created, we only want to update the values
	// pointed at, not the pointer itself.
	*s.config = *config
	if args.RawConfig != nil {
		*s.rawConfig = *args.RawConfig
	}
	if args.FleetConfig != nil {
		*s.fleetConfig = *args.FleetConfig
	}
	if args.Acker != nil {
		*s.acker = *args.Acker
	}
	s.pipeline = args.Pipeline
	s.namespace = args.Namespace
	if args.Beat != nil {
		*s.beat = *args.Beat
	}
	if args.Logger != nil {
		*s.logger = *args.Logger
	}
	if args.Tracer != nil {
		*s.tracer = *args.Tracer
	}
	if args.TracerServer != nil {
		*s.tracerServer = *args.TracerServer
	}
	s.wrapRunServer = args.WrapRunServer

	// Send config to telemetry.
	recordAPMServerConfig(s.config)

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

	if s.config.RumConfig.Enabled && s.config.RumConfig.SourceMapping.Enabled {
		store, err := newSourcemapStore(s.beat.Info, s.config.RumConfig.SourceMapping, s.fleetConfig)
		if err != nil {
			return err
		}
		if s.sourcemapStore == nil {
			s.sourcemapStore = store
		} else {
			*s.sourcemapStore = *store
		}
	} else {
		// TODO: We have to remember to disable the sourcemap if it's
		// no longer enabled
		s.sourcemapStore = nil
	}

	if s.status == runnerCreated {
		pipeline := pipetool.WithACKer(s.pipeline, s.acker)
		publisherConfig := &publish.PublisherConfig{
			Info:      s.beat.Info,
			Pipeline:  s.config.Pipeline,
			Namespace: s.namespace,
		}
		publisher, err := publish.NewPublisher(pipeline, s.tracer, publisherConfig)
		if err != nil {
			return err
		}
		s.publisher = publisher

		reporter := publisher.Send
		s.batchProcessor = &reporterBatchProcessor{reporter}
		if !s.config.Sampling.KeepUnsampled {
			// The server has been configured to discard unsampled
			// transactions. Make sure this is done just before calling
			// the publisher to avoid affecting aggregations.
			s.batchProcessor = modelprocessor.Chained{
				sampling.NewDiscardUnsampledBatchProcessor(),
				s.batchProcessor,
			}
		}

		server, err := newServer(
			s.logger,
			s.beat.Info,
			s.config,
			s.tracer,
			reporter,
			s.sourcemapStore,
			s.batchProcessor,
		)
		if err != nil {
			return err
		}
		s.server = server
	} else {
		// TODO: Bit weird to send identical arguments into newServer
		// and server.configure(). But I want newServer to return a
		// configured server, and configure() to configure an existing
		// one.
		if err := s.server.configure(
			s.logger,
			s.beat.Info,
			s.config,
			s.tracer,
			s.publisher.Send,
			s.sourcemapStore,
			s.batchProcessor,
		); err != nil {
			s.logger.Errorf("failed to configure server: %v", err)
			return err
		}
	}

	return nil
}

func (s *serverRunner) String() string {
	return "APMServer"
}

// Stop begins a timer, after which the server will shutdown. When the server
// is being restarted, Stop() and then Start() are called, but we cannot
// differentiate between that and only receiving Stop().
// Start a timer, and if we receive a message from the Start() method, do not
// stop the server.
// The channel in the timer still needs to be drained.
func (s *serverRunner) Stop() {
	t := time.NewTimer(10 * time.Second)
	go func() {
		select {
		case <-t.C:
			s.stopOnce.Do(s.cancelRunServerContext)
			s.Wait()
			s.status = runnerDone
		case <-s.restartc:
			// We've received a message from Start(), don't shut
			// the server down.
			s.logger.Info("restarting apm-server!")
			// Drain the channel
			if !t.Stop() {
				<-t.C
			}
		}
	}()
}

// Wait waits for the server to stop.
func (s *serverRunner) Wait() {
	s.wg.Wait()
}

// Start starts the server.
func (s *serverRunner) Start() {
	// If Start() is being called after Stop(), let the server know to not
	// shutdown.
	select {
	case s.restartc <- struct{}{}:
	default:
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.mu.Lock()

		switch s.status {
		case runnerCreated:
			s.status = runnerRunning
			s.mu.Unlock()
			s.run()
		case runnerRunning:
			s.mu.Unlock()
		case runnerDone:
			// TODO: We don't want this to happen.
		}
	}()
}

func (s *serverRunner) run() error {
	// When the publisher stops cleanly it will close its pipeline client,
	// calling the acker's Close method. We need to call Open for each new
	// publisher to ensure we wait for all clients and enqueued events to
	// be closed at shutdown time.
	s.acker.Open()
	defer s.publisher.Stop(s.backgroundContext)

	// TestPublishIntegration is failing because the params get modified as
	// they pass through various RunServers, before they finally (used to)
	// create the server.
	runServer := s.server.run
	if s.tracerServer != nil {
		runServer = runServerWithTracerServer(runServer, s.tracerServer, s.tracer)
	}
	if s.wrapRunServer != nil {
		// Wrap runServer function, enabling injection of
		// behaviour into the processing/reporting pipeline.
		runServer = s.wrapRunServer(runServer)
	}
	runServer = s.wrapRunServerWithPreprocessors(runServer)

	go func() {
		sigs := make(chan os.Signal, 1)
		defer close(sigs)
		signal.Notify(sigs, syscall.SIGUSR1)
		for {
			select {
			case <-sigs:
				rawcfg, err := common.LoadFile("./apm-server.dev.yml")
				if err != nil {
					fmt.Printf("failed to load apm-server.dev.yml! %v\n", err)
					continue
				}

				// Not sure how else to just grab the
				// apm-server section from the config
				integrationConfig, err := config.NewIntegrationConfig(rawcfg)
				if err != nil {
					fmt.Printf("failed to integration config apm-server.dev.yml! %v\n", err)
					continue
				}
				cfg := integrationConfig.APMServer
				params := s.params
				params.RawConfig = cfg
				s.params = params
				s.configure(params)
			case <-s.runServerContext.Done():
				return
			}
		}
	}()

	params := ServerParams{
		Info:           s.beat.Info,
		Config:         s.config,
		Managed:        s.beat.Manager != nil && s.beat.Manager.Enabled(),
		Namespace:      s.namespace,
		Logger:         s.logger,
		Tracer:         s.tracer,
		BatchProcessor: s.batchProcessor,
		SourcemapStore: s.sourcemapStore,
	}
	if err := runServer(s.runServerContext, params); err != nil {
		return err
	}
	return s.publisher.Stop(s.backgroundContext)
}

func (s *serverRunner) wrapRunServerWithPreprocessors(runServer RunServerFunc) RunServerFunc {
	processors := []model.BatchProcessor{
		modelprocessor.SetSystemHostname{},
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
