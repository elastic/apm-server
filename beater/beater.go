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
	"strings"
	"sync"

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
	esoutput "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/ingest/pipeline"
	logs "github.com/elastic/apm-server/log"
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

		esOutputCfg := elasticsearchOutputConfig(b)

		var err error
		bt.config, err = config.NewConfig(bt.rawConfig, esOutputCfg)
		if err != nil {
			return nil, err
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
	namespace     string
	wrapRunServer func(RunServerFunc) RunServerFunc
	waitPublished *waitPublishedAcker

	mutex      sync.Mutex // guards stopServer and stopped
	stopServer func()
	stopped    bool
}

// Run runs the APM Server, blocking until the beater's Stop method is called,
// or a fatal error occurs.
func (bt *beater) Run(b *beat.Beat) error {
	done := make(chan struct{})

	var reloadOnce sync.Once
	var reloadable = reload.ReloadableFunc(func(ucfg *reload.ConfigWithMeta) error {
		var err error
		// Elastic Agent might call ReloadableFunc many times, but we only need to act upon the first call,
		// during startup. This might change when APM Server is included in Fleet
		reloadOnce.Do(func() {
			defer close(done)

			integrationConfig, err := config.NewIntegrationConfig(ucfg.Config)
			if err != nil {
				bt.logger.Error("Could not parse integration configuration from Elastic Agent", err)
				return
			}

			var cfg *config.Config
			apmServerCommonConfig := integrationConfig.APMServer
			apmServerCommonConfig.Merge(common.MustNewConfigFrom(`{"data_streams.enabled": true}`))
			cfg, err = config.NewConfig(apmServerCommonConfig, elasticsearchOutputConfig(b))
			if err != nil {
				bt.logger.Error("Could not parse apm-server configuration from Elastic Agent ", err)
				return
			}

			bt.config = cfg
			bt.rawConfig = apmServerCommonConfig
			if integrationConfig.DataStream != nil {
				bt.namespace = integrationConfig.DataStream.Namespace
			}
			bt.logger.Info("Applying configuration from Elastic Agent... ")
		})
		return err
	})
	if b.Manager != nil && b.Manager.Enabled() {
		bt.logger.Info("Running under Elastic Agent, waiting for configuration... ")
		reload.Register.MustRegister("inputs", reloadable)
		<-done
	}

	// send configs to telemetry
	if err := recordRootConfig(b.Info, bt.rawConfig); err != nil {
		bt.logger.Errorf("Error recording telemetry data", err)
	}
	recordAPMServerConfig(bt.config)

	tracer, tracerServer, err := initTracing(b, bt.config, bt.logger)
	if err != nil {
		return err
	}
	if tracer != nil {
		defer tracer.Close()
		if tracerServer != nil {
			defer tracerServer.Close()
		}
	}

	runServer := runServer
	if tracerServer != nil {
		runServer = runServerWithTracerServer(runServer, tracerServer, tracer)
	}
	if bt.wrapRunServer != nil {
		// Wrap runServer function, enabling injection of
		// behaviour into the processing/reporting pipeline.
		runServer = bt.wrapRunServer(runServer)
	}

	publisher, err := bt.newPublisher(b, tracer)
	if err != nil {
		return err
	}

	// shutdownContext may be updated by stopServer below,
	// to initiate the shutdown timeout.
	shutdownContext := context.Background()
	var cancelShutdownContext context.CancelFunc
	defer func() {
		if cancelShutdownContext != nil {
			defer cancelShutdownContext()
		}
		publisher.Stop(shutdownContext)
		bt.waitPublished.Wait(shutdownContext)
	}()

	reporter := publisher.Send
	if !bt.config.Sampling.KeepUnsampled {
		// The server has been configured to discard unsampled
		// transactions. Make sure this is done just before calling
		// the publisher to avoid affecting aggregations.
		reporter = sampling.NewDiscardUnsampledReporter(reporter)
	}

	stopped := make(chan struct{})
	defer close(stopped)
	ctx, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()
	var stopOnce sync.Once
	stopServer := func() {
		stopOnce.Do(func() {
			if bt.config.ShutdownTimeout > 0 {
				shutdownContext, cancelShutdownContext = context.WithTimeout(
					shutdownContext, bt.config.ShutdownTimeout,
				)
			}
			cancelContext()
			<-stopped
		})
	}

	bt.mutex.Lock()
	if bt.stopped {
		bt.mutex.Unlock()
		return nil
	}
	bt.stopServer = stopServer
	bt.mutex.Unlock()

	return runServer(ctx, ServerParams{
		Info:     b.Info,
		Config:   bt.config,
		Logger:   bt.logger,
		Tracer:   tracer,
		Reporter: reporter,
	})
}

// checkConfig verifies the global configuration doesn't use unsupported settings
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
	var err error
	tracer := b.Instrumentation.Tracer()
	listener := b.Instrumentation.Listener()

	if !tracer.Active() && cfg != nil {
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
			return tracerServer.serve(ctx, args.Reporter)
		})
		g.Go(func() error {
			return runServer(ctx, args)
		})
		return g.Wait()
	}
}

func (bt *beater) newPublisher(b *beat.Beat, tracer *apm.Tracer) (*publish.Publisher, error) {
	transformConfig, err := newTransformConfig(b.Info, bt.config)
	if err != nil {
		return nil, err
	}
	publisherConfig := &publish.PublisherConfig{
		Info:            b.Info,
		Pipeline:        bt.config.Pipeline,
		Namespace:       bt.namespace,
		TransformConfig: transformConfig,
	}
	pipeline := pipetool.WithACKer(b.Publisher, bt.waitPublished)
	return publish.NewPublisher(pipeline, tracer, publisherConfig)
}

func newTransformConfig(beatInfo beat.Info, cfg *config.Config) (*transform.Config, error) {
	transformConfig := &transform.Config{
		DataStreams: cfg.DataStreams.Enabled,
		RUM: transform.RUMConfig{
			LibraryPattern:      regexp.MustCompile(cfg.RumConfig.LibraryPattern),
			ExcludeFromGrouping: regexp.MustCompile(cfg.RumConfig.ExcludeFromGrouping),
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
