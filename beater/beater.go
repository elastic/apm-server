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
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/cfgfile"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/ingest/pipeline"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/sampling"
)

var (
	errSetupDashboardRemoved = errors.New("setting 'setup.dashboards' has been removed")
)

// CreatorParams holds parameters for creating beat.Beaters.
type CreatorParams struct {
	// RunServer is used to run the APM Server.
	//
	// This should be set to beater.RunServer, or a function which wraps it.
	RunServer RunServerFunc
}

// NewCreator returns a new beat.Creator which creates beaters
// using the provided CreatorParams.
func NewCreator(args CreatorParams) beat.Creator {
	if args.RunServer == nil {
		panic("args.RunServer must be non-nil")
	}
	return func(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
		logger := logp.NewLogger(logs.Beater)
		if err := checkConfig(logger); err != nil {
			return nil, err
		}
		var esOutputCfg *common.Config
		if isElasticsearchOutput(b) {
			esOutputCfg = b.Config.Output.Config()
		}

		beaterConfig, err := config.NewConfig(b.Info.Version, ucfg, esOutputCfg)
		if err != nil {
			return nil, err
		}

		bt := &beater{
			config:    beaterConfig,
			stopped:   false,
			logger:    logger,
			runServer: args.RunServer,
		}

		// setup pipelines if explicitly directed to or setup --pipelines and config is not set at all
		shouldSetupPipelines := beaterConfig.Register.Ingest.Pipeline.IsEnabled() ||
			(b.InSetupCmd && beaterConfig.Register.Ingest.Pipeline.Enabled == nil)
		if isElasticsearchOutput(b) && shouldSetupPipelines {
			logger.Info("Registering pipeline callback")
			err := bt.registerPipelineCallback(b)
			if err != nil {
				return nil, err
			}
		} else {
			logger.Info("No pipeline callback registered")
		}
		return bt, nil
	}
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

type beater struct {
	config    *config.Config
	logger    *logp.Logger
	runServer RunServerFunc

	mutex      sync.Mutex // guards stopServer and stopped
	stopServer func()
	stopped    bool
}

// Run runs the APM Server, blocking until the beater's Stop method is called,
// or a fatal error occurs.
func (bt *beater) Run(b *beat.Beat) error {
	tracer, tracerServer, err := initTracer(b.Info, bt.config, bt.logger)
	if err != nil {
		return err
	}
	defer tracer.Close()

	runServer := bt.runServer
	if tracerServer != nil {
		// Self-instrumentation enabled, so running the APM Server
		// should run an internal server for receiving trace data.
		origRunServer := runServer
		runServer = func(ctx context.Context, args ServerParams) error {
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
				return origRunServer(ctx, args)
			})
			return g.Wait()
		}
	}

	publisher, err := publish.NewPublisher(b.Publisher, tracer, &publish.PublisherConfig{
		Info:            b.Info,
		ShutdownTimeout: bt.config.ShutdownTimeout,
		Pipeline:        bt.config.Pipeline,
	})
	if err != nil {
		return err
	}
	defer publisher.Stop()

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

func isElasticsearchOutput(b *beat.Beat) bool {
	return b.Config != nil && b.Config.Output.Name() == "elasticsearch"
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
	_, err := elasticsearch.RegisterConnectCallback(func(conn *eslegclient.Connection) error {
		return pipeline.RegisterPipelines(conn, overwrite, path)
	})
	return err
}

// Stop stops the beater gracefully.
func (bt *beater) Stop() {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()
	if bt.stopped || bt.stopServer == nil {
		return
	}
	bt.logger.Infof("stopping apm-server... waiting maximum of %v seconds for queues to drain",
		bt.config.ShutdownTimeout.Seconds())
	bt.stopServer()
	bt.stopped = true
}
