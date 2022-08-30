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
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.elastic.co/apm/module/apmgrpc/v2"
	"go.elastic.co/apm/module/apmhttp/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/licenser"
	esoutput "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/elastic-agent-libs/transport"
	"github.com/elastic/elastic-agent-libs/transport/tlscommon"
	"github.com/elastic/go-ucfg"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	javaattacher "github.com/elastic/apm-server/internal/beater/java_attacher"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/kibana"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/internal/model"
	"github.com/elastic/apm-server/internal/model/modelindexer"
	"github.com/elastic/apm-server/internal/model/modelprocessor"
	"github.com/elastic/apm-server/internal/publish"
	"github.com/elastic/apm-server/internal/sourcemap"
)

// CreatorParams holds parameters for creating beat.Beaters.
type CreatorParams struct {
	// Logger is a logger to use in Beaters created by the beat.Creator.
	//
	// If Logger is nil, logp.NewLogger will be used to create a new one.
	Logger *logp.Logger

	// WrapServer is optional, and if provided, will be called to wrap
	// the ServerParams and RunServerFunc used to run the APM Server.
	//
	// The WrapServer function may modify ServerParams, for example by
	// wrapping the BatchProcessor with additional processors. Similarly,
	// WrapServer may wrap the RunServerFunc to run additional goroutines
	// along with the server.
	//
	// WrapServer may keep a reference to the provided ServerParams's
	// BatchProcessor for asynchronous event publication, such as for
	// aggregated metrics. All other events (i.e. those decoded from
	// agent payloads) should be sent to the BatchProcessor in the
	// ServerParams provided to RunServerFunc; this BatchProcessor will
	// have rate-limiting, authorization, and data preprocessing applied.
	WrapServer WrapServerFunc
}

// NewCreator returns a new beat.Creator which creates beaters
// using the provided CreatorParams.
func NewCreator(args CreatorParams) beat.Creator {
	return func(b *beat.Beat, ucfg *agentconfig.C) (beat.Beater, error) {
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
			wrapServer:                args.WrapServer,
			waitPublished:             publish.NewWaitPublishedAcker(),
			outputConfigReloader:      newChanReloader(),
			libbeatMonitoringRegistry: monitoring.Default.GetRegistry("libbeat"),
		}

		var elasticsearchOutputConfig *agentconfig.C
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
	rawConfig                 *agentconfig.C
	config                    *config.Config
	logger                    *logp.Logger
	wrapServer                WrapServerFunc
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
	// Use `maxprocs` to change the GOMAXPROCS respecting any CFS quotas, if
	// set. This is necessary since the Go runtime will default to the number
	// of CPUs available in the  machine it's running in, however, when running
	// in a container or in a cgroup with resource limits, the disparity can be
	// extreme.
	// Having a significantly greater GOMAXPROCS set than the granted CFS quota
	// results in a significant amount of time spent "throttling", essentially
	// pausing the the running OS threads for the throttled period.
	// Since the quotas may be updated without restarting the process, the
	// GOMAXPROCS are adjusted every 30s.
	go adjustMaxProcs(ctx, 30*time.Second, diffInfof(bt.logger), bt.logger.Errorf)

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

	// add deprecation warning if running on a 32-bit system
	if runtime.GOARCH == "386" {
		bt.logger.Warn("deprecation notice: support for 32-bit system target architecture will be removed in an upcoming version")
	}

	reloader := reloader{
		runServerContext: ctx,
		args: sharedServerRunnerParams{
			Beat:                      b,
			WrapServer:                bt.wrapServer,
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
		reloader.mu.Lock()
		err := reloader.reload()
		reloader.mu.Unlock()
		if err != nil {
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
	rawConfig    *agentconfig.C
	outputConfig agentconfig.Namespace
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

	r.mu.Lock()
	defer r.mu.Unlock()
	r.rawConfig = integrationConfig.APMServer
	// Merge in datastream namespace passed in from apm integration
	if integrationConfig.DataStream != nil && integrationConfig.DataStream.Namespace != "" {
		c := agentconfig.MustNewConfigFrom(map[string]interface{}{
			"data_streams.namespace": integrationConfig.DataStream.Namespace,
		})
		if r.rawConfig, err = agentconfig.MergeConfigs(r.rawConfig, c); err != nil {
			return err
		}
	}
	r.fleetConfig = &integrationConfig.Fleet
	return r.reload()
}

func (r *reloader) reloadOutput(config *reload.ConfigWithMeta) error {
	var outputConfig agentconfig.Namespace
	if config != nil {
		if err := config.Config.Unpack(&outputConfig); err != nil {
			return err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outputConfig = outputConfig
	return r.reload()
}

// reload creates a new serverRunner, launches it in a new goroutine, waits
// for it to have successfully started and returns after waiting for the previous
// serverRunner (if any) to exit. Calls to reload must be sycnhronized explicitly
// by acquiring reloader#mu by callers.
func (r *reloader) reload() error {
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

	// Wait for the new runner to start; this avoids the race condition in updating
	// the monitoring#Deafult global registry inside the runner due to two reloads,
	// one for the inputs and the other for the elasticsearch output
	select {
	case <-runner.done:
		return errors.New("runner exited unexpectedly")
	case <-runner.started:
		// runner has started
	}

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
	started                chan struct{}
	done                   chan struct{}

	pipeline                  beat.PipelineConnector
	acker                     *publish.WaitPublishedAcker
	namespace                 string
	config                    *config.Config
	rawConfig                 *agentconfig.C
	elasticsearchOutputConfig *agentconfig.C
	fleetConfig               *config.Fleet
	beat                      *beat.Beat
	logger                    *logp.Logger
	tracer                    *apm.Tracer
	tracerServer              *tracerServer
	wrapServer                WrapServerFunc
	libbeatMonitoringRegistry *monitoring.Registry
}

type serverRunnerParams struct {
	sharedServerRunnerParams

	RawConfig    *agentconfig.C
	FleetConfig  *config.Fleet
	OutputConfig agentconfig.Namespace
}

type sharedServerRunnerParams struct {
	Beat                      *beat.Beat
	WrapServer                WrapServerFunc
	Logger                    *logp.Logger
	Tracer                    *apm.Tracer
	TracerServer              *tracerServer
	Acker                     *publish.WaitPublishedAcker
	LibbeatMonitoringRegistry *monitoring.Registry
}

func newServerRunner(ctx context.Context, args serverRunnerParams) (*serverRunner, error) {
	var esOutputConfig *agentconfig.C
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
		started:                make(chan struct{}),

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
		wrapServer:                args.WrapServer,
		libbeatMonitoringRegistry: args.LibbeatMonitoringRegistry,
	}, nil
}

func (s *serverRunner) run(listener net.Listener) error {
	defer close(s.done)

	// Send config to telemetry.
	recordAPMServerConfig(s.config)

	var kibanaClient kibana.Client
	if s.config.Kibana.Enabled {
		kibanaClient = kibana.NewConnectingClient(s.config.Kibana.ClientConfig)
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
	drain := make(chan struct{})
	g.Go(func() error {
		if err := s.waitReady(ctx, kibanaClient); err != nil {
			// One or more preconditions failed; drop events.
			close(drain)
			return errors.Wrap(err, "error waiting for server to be ready")
		}
		// All preconditions have been met; start indexing documents
		// into elasticsearch.
		close(publishReady)
		return nil
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
		transport := &waitReadyRoundTripper{Transport: httpTransport, ready: publishReady, drain: drain}
		return elasticsearch.NewClientParams(elasticsearch.ClientParams{
			Config:    cfg,
			Transport: transport,
			RetryOnError: func(_ *http.Request, err error) bool {
				return !errors.Is(err, errServerShuttingDown)
			},
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

	authenticator, err := auth.NewAuthenticator(s.config.AgentAuth)
	if err != nil {
		return err
	}

	ratelimitStore, err := ratelimit.NewStore(
		s.config.AgentAuth.Anonymous.RateLimit.IPLimit,
		s.config.AgentAuth.Anonymous.RateLimit.EventLimit,
		3, // burst mulitiplier
	)
	if err != nil {
		return err
	}

	// Note that we intentionally do not use a grpc.Creds ServerOption
	// even if TLS is enabled, as TLS is handled by the net/http server.
	gRPCLogger := s.logger.Named("grpc")
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		apmgrpc.NewUnaryServerInterceptor(apmgrpc.WithRecovery(), apmgrpc.WithTracer(s.tracer)),
		interceptors.ClientMetadata(),
		interceptors.Logging(gRPCLogger),
		interceptors.Metrics(gRPCLogger),
		interceptors.Timeout(),
		interceptors.Auth(authenticator),
		interceptors.AnonymousRateLimit(ratelimitStore),
	))

	// Create the BatchProcessor chain that is used to process all events,
	// including the metrics aggregated by APM Server.
	finalBatchProcessor, closeFinalBatchProcessor, err := s.newFinalBatchProcessor(newElasticsearchClient)
	if err != nil {
		return err
	}
	batchProcessor := modelprocessor.Chained{
		// Ensure all events have observer.*, ecs.*, and data_stream.* fields added,
		// and are counted in metrics. This is done in the final processors to ensure
		// aggregated metrics are also processed.
		newObserverBatchProcessor(s.beat.Info),
		model.ProcessBatchFunc(ecsVersionBatchProcessor),
		&modelprocessor.SetDataStream{Namespace: s.namespace},
		modelprocessor.NewEventCounter(monitoring.Default.GetRegistry("apm-server")),

		// The server always drops non-RUM unsampled transactions. We store RUM unsampled
		// transactions as they are needed by the User Experience app, which performs
		// aggregations over dimensions that are not available in transaction metrics.
		//
		// It is important that this is done just before calling the publisher to
		// avoid affecting aggregations.
		modelprocessor.NewDropUnsampled(false /* don't drop RUM unsampled transactions*/),
		modelprocessor.DroppedSpansStatsDiscarder{},
		finalBatchProcessor,
	}

	serverParams := ServerParams{
		UUID:                   s.beat.Info.ID,
		Config:                 s.config,
		Managed:                s.beat.Manager != nil && s.beat.Manager.Enabled(),
		Namespace:              s.namespace,
		Logger:                 s.logger,
		Tracer:                 s.tracer,
		Authenticator:          authenticator,
		RateLimitStore:         ratelimitStore,
		BatchProcessor:         batchProcessor,
		SourcemapFetcher:       sourcemapFetcher,
		PublishReady:           publishReady,
		KibanaClient:           kibanaClient,
		NewElasticsearchClient: newElasticsearchClient,
		GRPCServer:             grpcServer,
	}
	if s.wrapServer != nil {
		// Wrap the serverParams and runServer function, enabling
		// injection of behaviour into the processing chain.
		serverParams, runServer, err = s.wrapServer(serverParams, runServer)
		if err != nil {
			return err
		}
	}

	// Add pre-processing batch processors to the beginning of the chain,
	// applying only to the events that are decoded from agent/client payloads.
	preBatchProcessors := modelprocessor.Chained{
		// Add a model processor that rate limits, and checks authorization for the
		// agent and service for each event. These must come at the beginning of the
		// processor chain.
		model.ProcessBatchFunc(rateLimitBatchProcessor),
		model.ProcessBatchFunc(authorizeEventIngestProcessor),

		// Pre-process events before they are sent to the final processors for
		// aggregation, sampling, and indexing.
		modelprocessor.SetHostHostname{},
		modelprocessor.SetServiceNodeName{},
		modelprocessor.SetMetricsetName{},
		modelprocessor.SetGroupingKey{},
		modelprocessor.SetErrorMessage{},
		modelprocessor.SetUnknownSpanType{},
	}
	if s.config.DefaultServiceEnvironment != "" {
		preBatchProcessors = append(preBatchProcessors, &modelprocessor.SetDefaultServiceEnvironment{
			DefaultServiceEnvironment: s.config.DefaultServiceEnvironment,
		})
	}
	serverParams.BatchProcessor = append(preBatchProcessors, serverParams.BatchProcessor)

	g.Go(func() error {
		return runServer(ctx, serverParams)
	})

	// Signal that the runner has started
	close(s.started)

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

// newFinalBatchProcessor returns the final model.BatchProcessor that publishes events,
// and a cleanup function which should be called on server shutdown. If the output is
// "elasticsearch", then we use modelindexer; otherwise we use the libbeat publisher.
func (s *serverRunner) newFinalBatchProcessor(
	newElasticsearchClient func(cfg *elasticsearch.Config) (elasticsearch.Client, error),
) (model.BatchProcessor, func(context.Context) error, error) {
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
		MaxRequests           int           `config:"max_requests"`
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
		MaxRequests:      esConfig.MaxRequests,
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
	monitoring.Default.Remove("output")
	monitoring.NewFunc(monitoring.Default, "output.elasticsearch.bulk_requests", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		stats := indexer.Stats()
		v.OnKey("available")
		v.OnInt(stats.AvailableBulkRequests)
		v.OnKey("completed")
		v.OnInt(stats.BulkRequests)
	})
	return indexer, indexer.Close, nil
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

		fleetServerURLs := make([]*url.URL, len(fleetCfg.Hosts))
		for i, host := range fleetCfg.Hosts {
			urlString, err := common.MakeURL(fleetCfg.Protocol, "", host, 8220)
			if err != nil {
				return nil, err
			}
			u, err := url.Parse(urlString)
			if err != nil {
				return nil, err
			}
			fleetServerURLs[i] = u
		}

		artifactRefs := make([]sourcemap.FleetArtifactReference, len(cfg.Metadata))
		for i, meta := range cfg.Metadata {
			artifactRefs[i] = sourcemap.FleetArtifactReference{
				ServiceName:        meta.ServiceName,
				ServiceVersion:     meta.ServiceVersion,
				BundleFilepath:     meta.BundleFilepath,
				FleetServerURLPath: meta.SourceMapURL,
			}
		}

		return sourcemap.NewFleetFetcher(
			&client,
			fleetCfg.AccessAPIKey,
			fleetServerURLs,
			artifactRefs,
		)
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

type logf func(string, ...interface{})

func adjustMaxProcs(ctx context.Context, d time.Duration, infof, errorf logf) error {
	setMaxProcs := func() {
		if _, err := maxprocs.Set(maxprocs.Logger(infof)); err != nil {
			errorf("failed to set GOMAXPROCS: %v", err)
		}
	}
	// set the gomaxprocs immediately.
	setMaxProcs()
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			setMaxProcs()
		}
	}
}

func diffInfof(logger *logp.Logger) logf {
	var last string
	return func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		if msg != last {
			logger.Info(msg)
			last = msg
		}
	}
}
