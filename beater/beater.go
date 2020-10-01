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
		logger := logp.NewLogger(logs.Beater)
		if err := checkConfig(logger); err != nil {
			return nil, err
		}
		bt := &beater{
			rawConfig:     ucfg,
			stopped:       false,
			logger:        logger,
			wrapRunServer: args.WrapRunServer,
		}

		esOutputCfg := elasticsearchOutputConfig(b)

		var err error
		bt.config, err = config.NewConfig(bt.rawConfig, esOutputCfg)
		if err != nil {
			return nil, err
		}

		// setup pipelines if explicitly directed to or setup --pipelines and config is not set at all,
		// and apm-server is not running supervised by Elastic Agent
		shouldSetupPipelines := bt.config.Register.Ingest.Pipeline.IsEnabled() ||
			(b.InSetupCmd && bt.config.Register.Ingest.Pipeline.Enabled == nil)
		runningUnderElasticAgent := b.Manager != nil && b.Manager.Enabled()

		if esOutputCfg != nil && shouldSetupPipelines && !runningUnderElasticAgent {
			bt.logger.Info("Registering pipeline callback")
			err := bt.registerPipelineCallback(b)
			if err != nil {
				return nil, err
			}
		} else {
			bt.logger.Info("No pipeline callback registered")
		}

		return bt, nil
	}
}

type beater struct {
	rawConfig     *common.Config
	config        *config.Config
	logger        *logp.Logger
	wrapRunServer func(RunServerFunc) RunServerFunc

	mutex      sync.Mutex // guards stopServer and stopped
	stopServer func()
	stopped    bool
}

var once sync.Once

// Run runs the APM Server, blocking until the beater's Stop method is called,
// or a fatal error occurs.
func (bt *beater) Run(b *beat.Beat) error {

	done := make(chan struct{})

	var reloadable = reload.ReloadableFunc(func(ucfg *reload.ConfigWithMeta) error {
		var err error
		// Elastic Agent might call ReloadableFunc many times, but we only need to act upon the first call,
		// during startup. This might change when APM Server is included in Fleet
		once.Do(func() {
			defer close(done)
			var cfg *config.Config
			cfg, err = config.NewConfig(ucfg.Config, elasticsearchOutputConfig(b))
			if err != nil {
				bt.logger.Warn("Could not parse configuration from Elastic Agent ", err)
			}
			bt.config = cfg
			bt.rawConfig = ucfg.Config
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
	recordConfigs(b.Info, bt.config, bt.rawConfig, bt.logger)

	tracer, tracerServer, err := bt.initTracing(b)
	if err != nil {
		return err
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

	publisher, err := newPublisher(b, bt.config, tracer)
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
	if b.Config != nil && b.Config.Output.Name() == "elasticsearch" {
		return b.Config.Output.Config()
	}
	return nil
}

func (bt *beater) registerPipelineCallback(b *beat.Beat) error {
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

func (bt *beater) initTracing(b *beat.Beat) (*apm.Tracer, *tracerServer, error) {
	var err error
	tracer := b.Instrumentation.Tracer()
	listener := b.Instrumentation.Listener()

	if !tracer.Active() && bt.config != nil {
		tracer, listener, err = initLegacyTracer(b.Info, bt.config)
		if err != nil {
			return nil, nil, err
		}
	}

	tracerServer := newTracerServer(bt.config, listener)
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
			defer tracerServer.stop()
			<-ctx.Done()
			// Close the tracer now to prevent the server
			// from waiting for more events during graceful
			// shutdown.
			tracer.Close()
			return nil
		})
		g.Go(func() error {
			return tracerServer.serve(args.Reporter)
		})
		g.Go(func() error {
			return runServer(ctx, args)
		})
		return g.Wait()
	}
}

func newPublisher(b *beat.Beat, cfg *config.Config, tracer *apm.Tracer) (*publish.Publisher, error) {
	transformConfig, err := newTransformConfig(b.Info, cfg)
	if err != nil {
		return nil, err
	}
	return publish.NewPublisher(b.Publisher, tracer, &publish.PublisherConfig{
		Info:            b.Info,
		Pipeline:        cfg.Pipeline,
		TransformConfig: transformConfig,
	})
}

func newTransformConfig(beatInfo beat.Info, cfg *config.Config) (*transform.Config, error) {
	transformConfig := &transform.Config{
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
