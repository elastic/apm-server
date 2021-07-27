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
			pipeline:      b.Publisher,
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
	rawConfig      *common.Config
	config         *config.Config
	logger         *logp.Logger
	wrapRunServer  func(RunServerFunc) RunServerFunc
	waitPublished  *waitPublishedAcker
	pipeline       beat.Pipeline
	publisher      *publish.Publisher
	namespace      string
	args           sharedServerParams
	server         *server
	sourcemapStore *sourcemap.Store
	batchProcessor model.BatchProcessor

	ctx context.Context

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

	ctx, cancel := context.WithCancel(ctx)
	bt.ctx = ctx
	args := sharedServerParams{
		Beat:          b,
		WrapRunServer: bt.wrapRunServer,
		Logger:        bt.logger,
		Tracer:        tracer,
		TracerServer:  tracerServer,
		Acker:         bt.waitPublished,
	}

	if b.Manager != nil && b.Manager.Enabled() {
		// Management enabled, register beater as reloadable.
		bt.stopServer = func() {
			defer close(done)
			defer closeTracer()
			if bt.config.ShutdownTimeout > 0 {
				time.AfterFunc(bt.config.ShutdownTimeout, cancelContext)
			}
			cancel()
		}
		reload.Register.MustRegister("apm-server", bt)
	} else {
		// Management disabled, use statically defined config.
		err := bt.configure(serverParams{
			sharedServerParams: args,
			Pipeline:           b.Publisher,
			Namespace:          "default",
			RawConfig:          bt.rawConfig,
		})
		if err != nil {
			return nil, err
		}
		bt.stopServer = func() {
			if bt.config.ShutdownTimeout > 0 {
				time.AfterFunc(bt.config.ShutdownTimeout, cancelContext)
			}
			cancel()
		}

		go func() {
			bt.run(ctx)
			defer close(done)
			defer closeTracer()
		}()
	}
	return done, nil
}

func (b *beater) Reload(cfg *reload.ConfigWithMeta) error {
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
	params := serverParams{
		sharedServerParams: b.args,
		Namespace:          namespace,
		// TODO: We used to have a beat.PipelineConnector
		// Is this always the same as the beat.Pipeline we use elsewhere?
		Pipeline:    b.pipeline,
		RawConfig:   apmServerCommonConfig,
		FleetConfig: &integrationConfig.Fleet,
	}

	// TODO: This lock is here and not configure() because we're locked in
	// start(), where we also call configure(), which was causing a
	// deadlock. We're splitting configuration between two functions, try
	// to unify it.
	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.configure(params)
}

func (b *beater) configure(args serverParams) error {
	config, err := config.NewConfig(args.RawConfig, elasticsearchOutputConfig(args.Beat))
	if err != nil {
		return err
	}

	// once the server has been created, we only want to update the values
	// pointed at, not the pointer itself.
	*b.config = *config
	b.namespace = args.Namespace
	if args.RawConfig != nil {
		*b.rawConfig = *args.RawConfig
	}
	b.args = args.sharedServerParams

	// Send config to telemetry.
	recordAPMServerConfig(b.config)

	cfg := ucfg.Config(*b.rawConfig)
	parentCfg := cfg.Parent()
	// Check for an environment variable set when running in a cloud environment
	if eac := os.Getenv("ELASTIC_AGENT_CLOUD"); eac != "" && b.config.Kibana.Enabled {
		// Don't block server startup sending the config.
		go func() {
			c := kibana_client.NewConnectingClient(&b.config.Kibana)
			if err := kibana_client.SendConfig(b.ctx, c, parentCfg); err != nil {
				b.logger.Infof("failed to upload config to kibana: %v", err)
			}
		}()
	}

	var sourcemapStore *sourcemap.Store
	if b.config.RumConfig.Enabled && b.config.RumConfig.SourceMapping.Enabled {
		sourcemapStore, err = newSourcemapStore(
			args.Beat.Info,
			b.config.RumConfig.SourceMapping,
			args.FleetConfig,
		)
		if err != nil {
			return err
		}
	}
	b.sourcemapStore = sourcemapStore

	pipeline := pipetool.WithACKer(args.Pipeline, args.Acker)
	publisherConfig := &publish.PublisherConfig{
		Info:      args.Beat.Info,
		Pipeline:  b.config.Pipeline,
		Namespace: args.Namespace,
	}
	publisher, err := publish.NewPublisher(pipeline, args.Tracer, publisherConfig)
	if err != nil {
		return err
	}
	b.publisher = publisher

	reporter := publisher.Send
	batchProcessor := model.BatchProcessor(&reporterBatchProcessor{reporter})
	if !b.config.Sampling.KeepUnsampled {
		// The server has been configured to discard unsampled
		// transactions. Make sure this is done just before calling
		// the publisher to avoid affecting aggregations.
		batchProcessor = modelprocessor.Chained{
			sampling.NewDiscardUnsampledBatchProcessor(),
			batchProcessor,
		}
	}
	b.batchProcessor = batchProcessor

	if b.server == nil || b.server.status != serverRunning {
		server, err := newServer(
			b.logger,
			args.Beat.Info,
			b.config,
			args.Tracer,
			reporter,
			sourcemapStore,
			batchProcessor,
		)
		if err != nil {
			return err
		}
		b.server = server
	} else {
		// TODO: Bit weird to send identical arguments into newServer
		// and server.configure(). But I want newServer to return a
		// configured server, and configure() to configure an existing
		// one.
		if err := b.server.configure(
			b.logger,
			args.Beat.Info,
			b.config,
			args.Tracer,
			reporter,
			sourcemapStore,
			batchProcessor,
		); err != nil {
			b.logger.Errorf("failed to configure server: %v", err)
			return err
		}
	}

	return nil
}

func (b *beater) run(ctx context.Context) error {
	// When the publisher stops cleanly it will close its pipeline client,
	// calling the acker's Close method. We need to call Open for each new
	// publisher to ensure we wait for all clients and enqueued events to
	// be closed at shutdown time.
	b.args.Acker.Open()
	defer b.publisher.Stop(ctx)

	// TestPublishIntegration is failing because the params get modified as
	// they pass through various RunServers, before they finally (used to)
	// create the server.
	runServer := b.server.run
	if b.args.TracerServer != nil {
		runServer = runServerWithTracerServer(
			runServer,
			b.args.TracerServer,
			b.args.Tracer,
		)
	}
	if b.args.WrapRunServer != nil {
		// Wrap runServer function, enabling injection of
		// behaviour into the processing/reporting pipeline.
		runServer = b.args.WrapRunServer(runServer)
	}
	runServer = wrapRunServerWithPreprocessors(
		b.config,
		b.namespace,
		runServer,
	)

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
				fmt.Println("reloading apm-server")

				integrationConfig, err := config.NewIntegrationConfig(rawcfg)
				if err != nil {
					fmt.Println("failed to get apm config:", err)
					continue
				}
				var namespace string
				if integrationConfig.DataStream != nil {
					namespace = integrationConfig.DataStream.Namespace
				}
				apmServerCommonConfig := integrationConfig.APMServer
				params := serverParams{
					sharedServerParams: b.args,
					Namespace:          namespace,
					// TODO: We used to have a beat.PipelineConnector
					// Is this always the same as the beat.Pipeline we use elsewhere?
					Pipeline:  b.pipeline,
					RawConfig: apmServerCommonConfig,
				}

				if err := b.configure(params); err != nil {
					fmt.Println("got err while reloading:", err)
					continue
				}
			case <-b.ctx.Done():
				return
			}
		}
	}()

	params := ServerParams{
		Info:           b.args.Beat.Info,
		Config:         b.config,
		Managed:        b.args.Beat.Manager != nil && b.args.Beat.Manager.Enabled(),
		Namespace:      b.namespace,
		Logger:         b.logger,
		Tracer:         b.args.Tracer,
		BatchProcessor: b.batchProcessor,
		SourcemapStore: b.sourcemapStore,
	}
	// TODO: Re-add the runserver/background context differentiation
	if err := runServer(ctx, params); err != nil {
		return err
	}
	return b.publisher.Stop(ctx)
}

func wrapRunServerWithPreprocessors(
	config *config.Config,
	namespace string,
	runServer RunServerFunc,
) RunServerFunc {
	processors := []model.BatchProcessor{
		modelprocessor.SetHostHostname{},
		modelprocessor.SetServiceNodeName{},
		modelprocessor.SetMetricsetName{},
		modelprocessor.SetGroupingKey{},
	}
	if config.DefaultServiceEnvironment != "" {
		processors = append(processors, &modelprocessor.SetDefaultServiceEnvironment{
			DefaultServiceEnvironment: config.DefaultServiceEnvironment,
		})
	}
	if config.DataStreams.Enabled {
		processors = append(processors, &modelprocessor.SetDataStream{
			Namespace: namespace,
		})
	}
	return WrapRunServerWithProcessors(runServer, processors...)
}

type serverParams struct {
	sharedServerParams

	Namespace   string
	Pipeline    beat.PipelineConnector
	RawConfig   *common.Config
	FleetConfig *config.Fleet
}

type sharedServerParams struct {
	Beat          *beat.Beat
	WrapRunServer func(RunServerFunc) RunServerFunc
	Logger        *logp.Logger
	Tracer        *apm.Tracer
	TracerServer  *tracerServer
	Acker         *waitPublishedAcker
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
