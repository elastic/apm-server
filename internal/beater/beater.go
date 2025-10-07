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
	"errors"
	"fmt"
	"hash"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/dustin/go-humanize"
	"go.elastic.co/apm/module/apmotel/v2"
	"go.elastic.co/apm/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/esleg/eslegclient"
	"github.com/elastic/beats/v7/libbeat/instrumentation"
	"github.com/elastic/beats/v7/libbeat/licenser"
	"github.com/elastic/beats/v7/libbeat/outputs"
	esoutput "github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
	"github.com/elastic/beats/v7/libbeat/publisher/pipeline"
	"github.com/elastic/beats/v7/libbeat/publisher/pipetool"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-ucfg"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/fips140"
	"github.com/elastic/apm-server/internal/kibana"
	srvmodelprocessor "github.com/elastic/apm-server/internal/model/modelprocessor"
	"github.com/elastic/apm-server/internal/publish"
	"github.com/elastic/apm-server/internal/sourcemap"
	"github.com/elastic/apm-server/internal/version"
)

// Runner initialises and runs and orchestrates the APM Server
// HTTP and gRPC servers, event processing pipeline, and output.
type Runner struct {
	wrapServer WrapServerFunc
	logger     *logp.Logger
	rawConfig  *agentconfig.C

	config                    *config.Config
	outputConfig              agentconfig.Namespace
	elasticsearchOutputConfig *agentconfig.C

	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	metricGatherer *apmotel.Gatherer
	beatMonitoring beat.Monitoring
	listener       net.Listener
}

// RunnerParams holds parameters for NewRunner.
type RunnerParams struct {
	// Config holds the full, raw, configuration, including apm-server.*
	// and output.* attributes.
	Config *agentconfig.C

	// Logger holds a logger to use for logging throughout the APM Server.
	Logger *logp.Logger

	// TracerProvider holds a trace.TracerProvider that can be used for
	// creating traces.
	TracerProvider trace.TracerProvider

	// MeterProvider holds a metric.MeterProvider that can be used for
	// creating metrics.
	MeterProvider metric.MeterProvider

	// MetricsGatherer holds an apmotel.Gatherer
	MetricsGatherer *apmotel.Gatherer

	// BeatMonitoring holds beat monitoring
	BeatMonitoring beat.Monitoring

	// WrapServer holds an optional WrapServerFunc, for wrapping the
	// ServerParams and RunServerFunc used to run the APM Server.
	//
	// If WrapServer is nil, no wrapping will occur.
	WrapServer WrapServerFunc
}

// NewRunner returns a new Runner that runs APM Server with the given parameters.
func NewRunner(args RunnerParams) (*Runner, error) {
	fips140.CheckFips()

	// the default tracer is leaking and its background
	// goroutine is spamming requests to this apm server (default endpoint)
	// If TLS is enabled it causes "http request sent to https endpoint".
	// Close the default tracer since it's not used.
	apm.DefaultTracer().Close()

	var unpackedConfig struct {
		APMServer  *agentconfig.C        `config:"apm-server"`
		Output     agentconfig.Namespace `config:"output"`
		DataStream struct {
			Namespace string `config:"namespace"`
		} `config:"data_stream"`
	}
	if err := args.Config.Unpack(&unpackedConfig); err != nil {
		return nil, err
	}

	var elasticsearchOutputConfig *agentconfig.C
	if unpackedConfig.Output.Name() == "elasticsearch" {
		elasticsearchOutputConfig = unpackedConfig.Output.Config()
	}
	cfg, err := config.NewConfig(unpackedConfig.APMServer, elasticsearchOutputConfig, args.Logger)
	if err != nil {
		return nil, err
	}
	if unpackedConfig.DataStream.Namespace != "" {
		cfg.DataStreams.Namespace = unpackedConfig.DataStream.Namespace
	}

	// We start the listener in the constructor, before Run is invoked,
	// to ensure zero downtime while any existing Runner is stopped.
	logger := args.Logger.Named("beater")
	listener, err := listen(cfg, logger)
	if err != nil {
		return nil, err
	}
	return &Runner{
		wrapServer: args.WrapServer,
		logger:     logger,
		rawConfig:  args.Config,

		config:                    cfg,
		outputConfig:              unpackedConfig.Output,
		elasticsearchOutputConfig: elasticsearchOutputConfig,

		tracerProvider: args.TracerProvider,
		meterProvider:  args.MeterProvider,
		metricGatherer: args.MetricsGatherer,
		beatMonitoring: args.BeatMonitoring,
		listener:       listener,
	}, nil
}

// Run runs the server, blocking until ctx is cancelled.
func (s *Runner) Run(ctx context.Context) error {
	defer s.listener.Close()
	g, ctx := errgroup.WithContext(ctx)
	meter := s.meterProvider.Meter("github.com/elastic/apm-server/internal/beater")

	// backgroundContext is a context to use in operations that should
	// block until shutdown, and will be cancelled after the shutdown
	// timeout.
	backgroundContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-ctx.Done()
		s.logger.Infof(
			"stopping apm-server... waiting maximum of %s for queues to drain",
			s.config.ShutdownTimeout,
		)
		time.AfterFunc(s.config.ShutdownTimeout, cancel)
	}()

	if s.config.Pprof.Enabled {
		// Profiling rates should be set once, early on in the program.
		runtime.SetBlockProfileRate(s.config.Pprof.BlockProfileRate)
		runtime.SetMutexProfileFraction(s.config.Pprof.MutexProfileRate)
		if s.config.Pprof.MemProfileRate > 0 {
			runtime.MemProfileRate = s.config.Pprof.MemProfileRate
		}
	}

	memLimitGB := processMemoryLimit(
		newCgroupReader(),
		sysMemoryReaderFunc(systemMemoryLimit),
		s.logger,
	)

	if s.config.MaxConcurrentDecoders == 0 {
		s.config.MaxConcurrentDecoders = maxConcurrentDecoders(memLimitGB)
		s.logger.Infof("MaxConcurrentDecoders set to %d based on 80 percent of %0.1fgb of memory",
			s.config.MaxConcurrentDecoders, memLimitGB,
		)
	}

	if s.config.Aggregation.MaxServices <= 0 {
		s.config.Aggregation.MaxServices = linearScaledValue(1_000, memLimitGB, 0)
		s.logger.Infof("Aggregation.MaxServices set to %d based on %0.1fgb of memory",
			s.config.Aggregation.MaxServices, memLimitGB,
		)
	}

	if s.config.Aggregation.ServiceTransactions.MaxGroups <= 0 {
		s.config.Aggregation.ServiceTransactions.MaxGroups = linearScaledValue(1_000, memLimitGB, 0)
		s.logger.Infof("Aggregation.ServiceTransactions.MaxGroups for service aggregation set to %d based on %0.1fgb of memory",
			s.config.Aggregation.ServiceTransactions.MaxGroups, memLimitGB,
		)
	}

	if s.config.Aggregation.Transactions.MaxGroups <= 0 {
		s.config.Aggregation.Transactions.MaxGroups = linearScaledValue(5_000, memLimitGB, 0)
		s.logger.Infof("Aggregation.Transactions.MaxGroups set to %d based on %0.1fgb of memory",
			s.config.Aggregation.Transactions.MaxGroups, memLimitGB,
		)
	}

	if s.config.Aggregation.ServiceDestinations.MaxGroups <= 0 {
		s.config.Aggregation.ServiceDestinations.MaxGroups = linearScaledValue(5_000, memLimitGB, 5_000)
		s.logger.Infof("Aggregation.ServiceDestinations.MaxGroups set to %d based on %0.1fgb of memory",
			s.config.Aggregation.ServiceDestinations.MaxGroups, memLimitGB,
		)
	}

	if s.config.Sampling.Tail.Enabled && s.config.Sampling.Tail.DatabaseCacheSize == 0 {
		// 1GB=16MB, 2GB=24MB, 4GB=40MB, ..., 32GB=264MB, 64GB=520MB
		s.config.Sampling.Tail.DatabaseCacheSize = uint64(linearScaledValue(8<<20, memLimitGB, 8<<20))
		s.logger.Infof("Sampling.Tail.DatabaseCacheSize set to %d based on %0.1fgb of memory",
			s.config.Sampling.Tail.DatabaseCacheSize, memLimitGB,
		)
	}

	// Send config to telemetry.
	recordAPMServerConfig(s.config, s.beatMonitoring.StateRegistry())

	var kibanaClient *kibana.Client
	if s.config.Kibana.Enabled {
		var err error
		kibanaClient, err = kibana.NewClient(s.config.Kibana.ClientConfig)
		if err != nil {
			return err
		}
	}

	// ELASTIC_AGENT_CLOUD is set when running in Elastic Cloud.
	inElasticCloud := os.Getenv("ELASTIC_AGENT_CLOUD") != ""
	if inElasticCloud {
		if s.config.Kibana.Enabled {
			go func() {
				if err := kibana.SendConfig(ctx, kibanaClient, (*ucfg.Config)(s.rawConfig)); err != nil {
					s.logger.Infof("failed to upload config to kibana: %v", err)
				}
			}()
		}
	}

	instrumentation, err := newInstrumentation(s.rawConfig, s.logger)
	if err != nil {
		return err
	}
	tracer := instrumentation.Tracer()
	tracerServerListener := instrumentation.Listener()
	if tracerServerListener != nil {
		defer tracerServerListener.Close()
	}
	defer tracer.Close()

	tracerProvider, err := apmotel.NewTracerProvider(apmotel.WithAPMTracer(tracer))
	if err != nil {
		return err
	}
	otel.SetTracerProvider(tracerProvider)

	s.tracerProvider = tracerProvider

	tracer.RegisterMetricsGatherer(s.metricGatherer)

	// Ensure the libbeat output and go-elasticsearch clients do not index
	// any events to Elasticsearch before the integration is ready.
	publishReady := make(chan struct{})
	drain := make(chan struct{})
	g.Go(func() error {
		if err := s.waitReady(ctx); err != nil {
			// One or more preconditions failed; drop events.
			close(drain)
			return fmt.Errorf("error waiting for server to be ready: %w", err)
		}
		// All preconditions have been met; start indexing documents
		// into elasticsearch.
		close(publishReady)
		return nil
	})
	callbackUUID, err := esoutput.RegisterConnectCallback(func(*eslegclient.Connection, *logp.Logger) error {
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
	newESClient := func(tp trace.TracerProvider) func(cfg *elasticsearch.Config, logger *logp.Logger) (*elasticsearch.Client, error) {
		return func(cfg *elasticsearch.Config, logger *logp.Logger) (*elasticsearch.Client, error) {
			httpTransport, err := elasticsearch.NewHTTPTransport(cfg, logger)
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
				Logger:         logger,
				TracerProvider: tp,
			})
		}
	}
	newElasticsearchClient := newESClient(s.tracerProvider)

	var sourcemapFetcher sourcemap.Fetcher
	if s.config.RumConfig.Enabled && s.config.RumConfig.SourceMapping.Enabled {
		fetcher, cancel, err := newSourcemapFetcher(
			s.config.RumConfig.SourceMapping,
			kibanaClient, newElasticsearchClient,
			s.tracerProvider,
			s.logger,
		)
		if err != nil {
			return err
		}
		defer cancel()
		sourcemapFetcher = fetcher
	}

	// Create the runServer function. We start with newBaseRunServer, and then
	// wrap depending on the configuration in order to inject behaviour.
	runServer := newBaseRunServer(s.listener)
	authenticator, err := auth.NewAuthenticator(s.config.AgentAuth, s.logger)
	if err != nil {
		return err
	}

	ratelimitStore, err := ratelimit.NewStore(
		s.config.AgentAuth.Anonymous.RateLimit.IPLimit,
		s.config.AgentAuth.Anonymous.RateLimit.EventLimit,
		3, // burst multiplier
	)
	if err != nil {
		return err
	}

	// Note that we intentionally do not use a grpc.Creds ServerOption
	// even if TLS is enabled, as TLS is handled by the net/http server.
	gRPCLogger := s.logger.Named("grpc")
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptors.Tracing(s.tracerProvider),
		interceptors.Recover(),
		interceptors.ClientMetadata(),
		interceptors.Logging(gRPCLogger),
		interceptors.Metrics(gRPCLogger, s.meterProvider),
		interceptors.Timeout(),
		interceptors.Auth(authenticator),
		interceptors.AnonymousRateLimit(ratelimitStore),
	))

	// Create the BatchProcessor chain that is used to process all events,
	// including the metrics aggregated by APM Server.
	finalBatchProcessor, closeFinalBatchProcessor, err := s.newFinalBatchProcessor(
		tracer, newElasticsearchClient, memLimitGB, s.logger, s.tracerProvider, s.meterProvider,
	)
	if err != nil {
		return err
	}
	transactionsDroppedCounter, err := meter.Int64Counter("apm-server.sampling.transactions_dropped")
	if err != nil {
		return err
	}
	batchProcessor := modelprocessor.Chained{
		// Ensure all events have observer.*, ecs.*, and data_stream.* fields added,
		// and are counted in metrics. This is done in the final processors to ensure
		// aggregated metrics are also processed.
		newObserverBatchProcessor(),
		&modelprocessor.SetDataStream{Namespace: s.config.DataStreams.Namespace},
		srvmodelprocessor.NewEventCounter(s.meterProvider),

		// The server always drops non-RUM unsampled transactions. We store RUM unsampled
		// transactions as they are needed by the User Experience app, which performs
		// aggregations over dimensions that are not available in transaction metrics.
		//
		// It is important that this is done just before calling the publisher to
		// avoid affecting aggregations.
		modelprocessor.NewDropUnsampled(false /* don't drop RUM unsampled transactions*/, func(i int64) {
			transactionsDroppedCounter.Add(context.Background(), i)
		}),
		finalBatchProcessor,
	}

	agentConfigFetcher, fetcherRunFunc, err := newAgentConfigFetcher(
		ctx,
		s.config,
		kibanaClient,
		newElasticsearchClient,
		s.tracerProvider,
		s.meterProvider,
		s.logger,
	)
	if err != nil {
		return err
	}
	if fetcherRunFunc != nil {
		g.Go(func() error {
			return fetcherRunFunc(ctx)
		})
	}

	agentConfigReporter := agentcfg.NewReporter(
		agentConfigFetcher,
		batchProcessor, 30*time.Second,
		s.logger,
	)
	g.Go(func() error {
		return agentConfigReporter.Run(ctx)
	})

	// Create the runServer function. We start with newBaseRunServer, and then
	// wrap depending on the configuration in order to inject behaviour.
	serverParams := ServerParams{
		Config:                 s.config,
		Namespace:              s.config.DataStreams.Namespace,
		Logger:                 s.logger,
		TracerProvider:         s.tracerProvider,
		MeterProvider:          s.meterProvider,
		Authenticator:          authenticator,
		RateLimitStore:         ratelimitStore,
		BatchProcessor:         batchProcessor,
		AgentConfig:            agentConfigReporter,
		SourcemapFetcher:       sourcemapFetcher,
		PublishReady:           publishReady,
		KibanaClient:           kibanaClient,
		NewElasticsearchClient: newESClient(tracenoop.NewTracerProvider()),
		GRPCServer:             grpcServer,
		Semaphore:              semaphore.NewWeighted(int64(s.config.MaxConcurrentDecoders)),
		BeatMonitoring:         s.beatMonitoring,
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
		modelpb.ProcessBatchFunc(rateLimitBatchProcessor),
		modelpb.ProcessBatchFunc(authorizeEventIngestProcessor),

		// Add a model processor that removes `event.received`, which is added by
		// apm-data, but which we don't yet map.
		modelprocessor.RemoveEventReceived{},

		// Pre-process events before they are sent to the final processors for
		// aggregation, sampling, and indexing.
		modelprocessor.SetHostHostname{},
		modelprocessor.SetServiceNodeName{},
		modelprocessor.SetGroupingKey{
			NewHash: func() hash.Hash {
				return xxhash.New()
			},
		},
		modelprocessor.SetErrorMessage{},
	}
	if s.config.DefaultServiceEnvironment != "" {
		preBatchProcessors = append(preBatchProcessors, &modelprocessor.SetDefaultServiceEnvironment{
			DefaultServiceEnvironment: s.config.DefaultServiceEnvironment,
		})
	}
	serverParams.BatchProcessor = append(preBatchProcessors, serverParams.BatchProcessor)

	// Start the main server and the optional server for self-instrumentation.
	g.Go(func() error {
		return runServer(ctx, serverParams)
	})
	closeTracerProcessor := func(context.Context) error { return nil }
	if tracerServerListener != nil {
		// use a batch processor without tracing to prevent the tracing processor from sending traces to itself
		finalTracerBatchProcessor, closeTracerFinalBatchProcessor, err := s.newFinalBatchProcessor(
			tracer, newESClient(tracenoop.NewTracerProvider()), memLimitGB, s.logger, tracenoop.NewTracerProvider(), metricnoop.NewMeterProvider(),
		)
		if err != nil {
			return err
		}

		tracerBatchProcessor := modelprocessor.Chained{
			newObserverBatchProcessor(),
			&modelprocessor.SetDataStream{Namespace: s.config.DataStreams.Namespace},
			finalTracerBatchProcessor,
		}

		tracerProcessor := append(preBatchProcessors, tracerBatchProcessor)
		tracerServer, err := newTracerServer(s.config, tracerServerListener, s.logger, tracerProcessor, serverParams.Semaphore, serverParams.MeterProvider)
		if err != nil {
			return fmt.Errorf("failed to create self-instrumentation server: %w", err)
		}
		go func() {
			if err := tracerServer.Serve(tracerServerListener); err != http.ErrServerClosed {
				s.logger.With(logp.Error(err)).Error("failed to shutdown tracer server")
			}
		}()

		closeTracerProcessor = func(ctx context.Context) error {
			return errors.Join(
				closeTracerFinalBatchProcessor(ctx),
				tracerServer.Shutdown(backgroundContext),
			)
		}
	}

	result := g.Wait()
	closeErr := closeFinalBatchProcessor(backgroundContext)
	closeTracerErr := closeTracerProcessor(backgroundContext)
	return errors.Join(result, closeErr, closeTracerErr)
}

// newInstrumentation is a thin wrapper around libbeat instrumentation that
// sets missing tracer configuration from elastic agent.
func newInstrumentation(rawConfig *agentconfig.C, logger *logp.Logger) (instrumentation.Instrumentation, error) {
	// This config struct contains missing fields from elastic agent APMConfig
	// https://github.com/elastic/elastic-agent/blob/main/internal/pkg/core/monitoring/config/config.go#L127
	// that are not directly handled by libbeat instrumentation below.
	//
	// Note that original config keys were additionally marshalled by
	// https://github.com/elastic/elastic-agent/blob/main/pkg/component/runtime/apm_config_mapper.go#L18
	// that's why some keys are different from the original APMConfig struct including "api_key" and "secret_token".
	var apmCfg struct {
		APIKey       string `config:"apikey"`
		SecretToken  string `config:"secrettoken"`
		GlobalLabels string `config:"globallabels"`
		TLS          struct {
			SkipVerify        bool   `config:"skipverify"`
			ServerCertificate string `config:"servercert"`
			ServerCA          string `config:"serverca"`
		} `config:"tls"`
		SamplingRate *float32 `config:"samplingrate"`
	}
	cfg, err := rawConfig.Child("instrumentation", -1)
	if err != nil || !cfg.Enabled() {
		// Fallback to instrumentation.New if the configs are not present or disabled.
		return instrumentation.New(rawConfig, "apm-server", version.VersionWithQualifier(), logger)
	}
	if err := cfg.Unpack(&apmCfg); err != nil {
		return nil, err
	}
	const (
		envAPIKey           = "ELASTIC_APM_API_KEY"
		envSecretToken      = "ELASTIC_APM_SECRET_TOKEN"
		envVerifyServerCert = "ELASTIC_APM_VERIFY_SERVER_CERT"
		envServerCert       = "ELASTIC_APM_SERVER_CERT"
		envCACert           = "ELASTIC_APM_SERVER_CA_CERT_FILE"
		envGlobalLabels     = "ELASTIC_APM_GLOBAL_LABELS"
		envSamplingRate     = "ELASTIC_APM_TRANSACTION_SAMPLE_RATE"
	)
	if apmCfg.APIKey != "" {
		os.Setenv(envAPIKey, apmCfg.APIKey)
		defer os.Unsetenv(envAPIKey)
	}
	if apmCfg.SecretToken != "" {
		os.Setenv(envSecretToken, apmCfg.SecretToken)
		defer os.Unsetenv(envSecretToken)
	}
	if apmCfg.TLS.SkipVerify {
		os.Setenv(envVerifyServerCert, "false")
		defer os.Unsetenv(envVerifyServerCert)
	}
	if apmCfg.TLS.ServerCertificate != "" {
		os.Setenv(envServerCert, apmCfg.TLS.ServerCertificate)
		defer os.Unsetenv(envServerCert)
	}
	if apmCfg.TLS.ServerCA != "" {
		os.Setenv(envCACert, apmCfg.TLS.ServerCA)
		defer os.Unsetenv(envCACert)
	}
	if len(apmCfg.GlobalLabels) > 0 {
		os.Setenv(envGlobalLabels, apmCfg.GlobalLabels)
		defer os.Unsetenv(envGlobalLabels)
	}
	if apmCfg.SamplingRate != nil {
		r := max(min(*apmCfg.SamplingRate, 1.0), 0.0)
		os.Setenv(envSamplingRate, strconv.FormatFloat(float64(r), 'f', -1, 32))
		defer os.Unsetenv(envSamplingRate)
	}
	return instrumentation.New(rawConfig, "apm-server", version.VersionWithQualifier(), logger)
}

func maxConcurrentDecoders(memLimitGB float64) uint {
	// Allow 128 concurrent decoders for each 1GB memory, limited to at most 2048.
	const max = 2048
	// Use 80% of the total memory limit to calculate decoders
	decoders := uint(128 * memLimitGB * 0.8)
	if decoders > max {
		return max
	}
	return decoders
}

// linearScaledValue calculates linearly scaled value based on memory limit using
// the formula y = (perGBIncrement * memLimitGB) + constant
func linearScaledValue(perGBIncrement, memLimitGB, constant float64) int {
	const maxMemGB = 64
	if memLimitGB > maxMemGB {
		memLimitGB = maxMemGB
	}
	return int(memLimitGB*perGBIncrement + constant)
}

// waitReady waits until the server is ready to index events.
func (s *Runner) waitReady(
	ctx context.Context,
) error {
	var preconditions []func(context.Context) error
	var esOutputClient *elasticsearch.Client
	if s.elasticsearchOutputConfig != nil {
		esConfig := elasticsearch.DefaultConfig()
		err := s.elasticsearchOutputConfig.Unpack(&esConfig)
		if err != nil {
			return err
		}
		esOutputClient, err = elasticsearch.NewClient(esConfig, s.logger)
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
				license, err := getElasticsearchLicense(ctx, esOutputClient)
				if err != nil {
					return fmt.Errorf("error getting Elasticsearch licensing information: %w", err)
				}
				if licenser.IsExpired(license) {
					return errors.New("the Elasticsearch license is expired")
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
			return queryClusterUUID(ctx, esOutputClient, s.beatMonitoring.StateRegistry())
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
	return waitReady(ctx, s.config.WaitReadyInterval, s.tracerProvider, s.logger, check)
}

// newFinalBatchProcessor returns the final model.BatchProcessor that publishes events,
// and a cleanup function which should be called on server shutdown. If the output is
// "elasticsearch", then we use docappender; otherwise we use the libbeat publisher.
func (s *Runner) newFinalBatchProcessor(
	tracer *apm.Tracer,
	newElasticsearchClient func(*elasticsearch.Config, *logp.Logger) (*elasticsearch.Client, error),
	memLimit float64,
	logger *logp.Logger,
	tp trace.TracerProvider,
	mp metric.MeterProvider,
) (modelpb.BatchProcessor, func(context.Context) error, error) {
	if s.elasticsearchOutputConfig == nil {
		s.beatMonitoring.StatsRegistry().Remove("libbeat")
		libbeatMonitoringRegistry := s.beatMonitoring.StatsRegistry().GetOrCreateRegistry("libbeat")
		return s.newLibbeatFinalBatchProcessor(tracer, libbeatMonitoringRegistry, logger)
	}

	stateRegistry := s.beatMonitoring.StateRegistry()
	outputRegistry := stateRegistry.GetOrCreateRegistry("output")
	outputRegistry.Clear()
	monitoring.NewString(outputRegistry, "name").Set("elasticsearch")

	// Create the docappender and Elasticsearch config
	appenderCfg, esCfg, err := s.newDocappenderConfig(tp, mp, memLimit)
	if err != nil {
		return nil, nil, err
	}
	client, err := newElasticsearchClient(esCfg, logger)
	if err != nil {
		return nil, nil, err
	}
	appender, err := docappender.New(client, appenderCfg)
	if err != nil {
		return nil, nil, err
	}

	return newDocappenderBatchProcessor(appender), appender.Close, nil
}

func (s *Runner) newDocappenderConfig(tp trace.TracerProvider, mp metric.MeterProvider, memLimit float64) (
	docappender.Config, *elasticsearch.Config, error,
) {
	esConfig := struct {
		*elasticsearch.Config `config:",inline"`
		FlushBytes            string        `config:"flush_bytes"`
		FlushInterval         time.Duration `config:"flush_interval"`
		MaxRequests           int           `config:"max_requests"`
		Scaling               struct {
			Enabled *bool `config:"enabled"`
		} `config:"autoscaling"`
	}{
		// Default to 1mib flushes, which is the default for go-docappender.
		FlushBytes:    "1 mib",
		FlushInterval: time.Second,
		Config:        elasticsearch.DefaultConfig(),
	}
	esConfig.MaxIdleConnsPerHost = 10

	if err := s.elasticsearchOutputConfig.Unpack(&esConfig); err != nil {
		return docappender.Config{}, nil, err
	}

	var flushBytes int
	if esConfig.FlushBytes != "" {
		b, err := humanize.ParseBytes(esConfig.FlushBytes)
		if err != nil {
			return docappender.Config{}, nil, fmt.Errorf("failed to parse flush_bytes: %w", err)
		}
		flushBytes = int(b)
	}
	minFlush := 24 * 1024
	if esConfig.CompressionLevel != 0 && flushBytes < minFlush {
		s.logger.Warnf("flush_bytes config value is too small (%d) and might be ignored by the indexer, increasing value to %d", flushBytes, minFlush)
		flushBytes = minFlush
	}
	var scalingCfg docappender.ScalingConfig
	if enabled := esConfig.Scaling.Enabled; enabled != nil {
		scalingCfg.Disabled = !*enabled
	}
	cfg := docappenderConfig(docappender.Config{
		CompressionLevel:     esConfig.CompressionLevel,
		FlushBytes:           flushBytes,
		FlushInterval:        esConfig.FlushInterval,
		TracerProvider:       tp,
		MeterProvider:        mp,
		MaxRequests:          esConfig.MaxRequests,
		Scaling:              scalingCfg,
		Logger:               zap.New(s.logger.Core(), zap.WithCaller(true)),
		RequireDataStream:    true,
		IncludeSourceOnError: docappender.False,
		// Use the output's max_retries to configure the go-docappender's
		// document level retries.
		MaxDocumentRetries:    esConfig.MaxRetries,
		RetryOnDocumentStatus: []int{429}, // Only retry "safe" 429 responses.
	}, memLimit, s.logger)
	if cfg.MaxRequests != 0 {
		esConfig.MaxIdleConnsPerHost = cfg.MaxRequests
	}

	return cfg, esConfig.Config, nil
}

func docappenderConfig(
	opts docappender.Config, memLimit float64, logger *logp.Logger,
) docappender.Config {
	const logMessage = "%s set to %d based on %0.1fgb of memory"
	// Use 80% of the total memory limit to calculate buffer size
	opts.DocumentBufferSize = int(1024 * memLimit * 0.8)
	if opts.DocumentBufferSize >= 61440 {
		opts.DocumentBufferSize = 61440
	}
	logger.Infof(logMessage,
		"docappender.DocumentBufferSize", opts.DocumentBufferSize, memLimit,
	)
	if opts.MaxRequests > 0 {
		logger.Infof("docappender.MaxRequests set to %d based on config value",
			opts.MaxRequests,
		)
		return opts
	}
	// This formula yields the following max requests for APM Server sized:
	// 1	2 	4	8	15	30
	// 10	12	14	19	28	46
	var multiplier float64
	if memLimit >= 8 {
		multiplier = 3
	} else {
		multiplier = 1.5
	}
	maxRequests := int(float64(10) + memLimit*multiplier)
	if maxRequests > 60 {
		maxRequests = 60
	}
	opts.MaxRequests = maxRequests
	logger.Infof(logMessage,
		"docappender.MaxRequests", opts.MaxRequests, memLimit,
	)
	return opts
}

func (s *Runner) newLibbeatFinalBatchProcessor(
	tracer *apm.Tracer,
	libbeatMonitoringRegistry *monitoring.Registry,
	logger *logp.Logger,
) (modelpb.BatchProcessor, func(context.Context) error, error) {
	// When the publisher stops cleanly it will close its pipeline client,
	// calling the acker's Close method and unblock Wait.
	acker := publish.NewWaitPublishedAcker()
	acker.Open()

	hostname, _ := os.Hostname()
	beatInfo := beat.Info{
		Beat:        "apm-server",
		IndexPrefix: "apm-server",
		Version:     version.VersionWithQualifier(),
		Hostname:    hostname,
		Name:        hostname,
		Logger:      logger,
	}

	stateRegistry := s.beatMonitoring.StateRegistry()
	stateRegistry.Remove("queue")
	monitors := pipeline.Monitors{
		Metrics:   libbeatMonitoringRegistry,
		Telemetry: stateRegistry,
		Logger:    logger.Named("publisher"),
		Tracer:    tracer,
	}
	outputFactory := func(stats outputs.Observer) (string, outputs.Group, error) {
		if !s.outputConfig.IsSet() {
			return "", outputs.Group{}, nil
		}
		outputName := s.outputConfig.Name()
		output, err := outputs.Load(nil, beatInfo, stats, outputName, s.outputConfig.Config())
		return outputName, output, err
	}
	var pipelineConfig pipeline.Config
	if err := s.rawConfig.Unpack(&pipelineConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to unpack libbeat pipeline config: %w", err)
	}
	pipeline, err := pipeline.Load(beatInfo, monitors, pipelineConfig, nopProcessingSupporter{}, outputFactory)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create libbeat output pipeline: %w", err)
	}
	pipelineConnector := pipetool.WithACKer(pipeline, acker)
	publisher, err := publish.NewPublisher(pipelineConnector)
	if err != nil {
		return nil, nil, err
	}
	stop := func(ctx context.Context) error {
		// clients need to be closed before running Close so
		// this method needs to be called after the publisher has
		// stopped
		defer pipeline.Close()
		if err := publisher.Stop(ctx); err != nil {
			return err
		}
		if !s.outputConfig.IsSet() {
			// No output defined, so the acker will never be closed.
			return nil
		}
		return acker.Wait(ctx)
	}
	return publisher, stop, nil
}

const sourcemapIndex = ".apm-source-map"

func newSourcemapFetcher(
	cfg config.SourceMapping,
	kibanaClient *kibana.Client,
	newElasticsearchClient func(*elasticsearch.Config, *logp.Logger) (*elasticsearch.Client, error),
	tp trace.TracerProvider,
	logger *logp.Logger,
) (sourcemap.Fetcher, context.CancelFunc, error) {
	esClient, err := newElasticsearchClient(cfg.ESConfig, logger)
	if err != nil {
		return nil, nil, err
	}

	var fetchers []sourcemap.Fetcher

	// start background sync job
	ctx, ctxCancel := context.WithCancel(context.Background())
	metadataFetcher, invalidationChan := sourcemap.NewMetadataFetcher(ctx, esClient, sourcemapIndex, tp, logger)
	cancel := func() {
		ctxCancel()
		<-invalidationChan
	}

	esFetcher := sourcemap.NewElasticsearchFetcher(esClient, sourcemapIndex, logger)
	size := 128
	cachingFetcher, err := sourcemap.NewBodyCachingFetcher(esFetcher, size, invalidationChan, logger)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	sourcemapFetcher := sourcemap.NewSourcemapFetcher(metadataFetcher, cachingFetcher, logger)

	fetchers = append(fetchers, sourcemapFetcher)

	if kibanaClient != nil {
		fetchers = append(fetchers, sourcemap.NewKibanaFetcher(kibanaClient, logger))
	}

	chained := sourcemap.NewChainedFetcher(fetchers, logger)

	return chained, cancel, nil
}

// TODO: This is copying behavior from libbeat:
// https://github.com/elastic/beats/blob/b9ced47dba8bb55faa3b2b834fd6529d3c4d0919/libbeat/cmd/instance/beat.go#L927-L950
// Remove this when cluster_uuid no longer needs to be queried from ES.
func queryClusterUUID(ctx context.Context, esClient *elasticsearch.Client, stateRegistry *monitoring.Registry) error {
	outputES := "outputs.elasticsearch"
	// Running under elastic-agent, the callback linked above is not
	// registered until later, meaning we need to check and instantiate the
	// registries if they don't exist.
	elasticsearchRegistry := stateRegistry.GetOrCreateRegistry(outputES)
	if elasticsearchRegistry == nil {
		// This can only happen if "outputs.elasticsearch" was already created as
		// a non-registry (scalar) value, but in that unlikely chance, let's still
		// report a comprehensible error.
		return fmt.Errorf("couldn't create registry: %v", outputES)
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

// processMemoryLimit obtains the memory limit for the APM Server process. Certain config
// values will be sized according to the maximum memory set for the server.
func processMemoryLimit(cgroups cgroupReader, sys sysMemoryReader, logger *logp.Logger) (memLimitGB float64) {
	var memLimit uint64
	if cgroups != nil {
		if limit, err := cgroupMemoryLimit(cgroups); err != nil {
			logger.Warn(err)
		} else {
			memLimit = limit
		}
	}
	if limit, err := sys.Limit(); err != nil {
		logger.Warn(err)
	} else {
		var fallback bool
		if memLimit <= 0 {
			logger.Info("no cgroups detected, falling back to total system memory")
			fallback = true
		}
		if memLimit > limit {
			logger.Info("cgroup memory limit exceed available memory, falling back to the total system memory")
			fallback = true
		}
		if fallback {
			// If no cgroup limit is set, return a fraction of the total memory
			// to have a margin of safety for other processes. The fraction value
			// of 0.625 is used to keep the 80% of the total system memory limit
			// to be 50% of the total for calculating the number of decoders.
			memLimit = uint64(float64(limit) * 0.625)
		}
	}
	// Convert the memory limit to gigabytes to calculate the config values.
	memLimitGB = float64(memLimit) / (1 << 30)
	if memLimitGB <= 0 {
		memLimitGB = 1
		logger.Infof(
			"failed to discover memory limit, default to %0.1fgb of memory",
			memLimitGB,
		)
	}
	return
}

type nopProcessingSupporter struct {
}

func (nopProcessingSupporter) Close() error {
	return nil
}

func (nopProcessingSupporter) Processors() []string {
	return nil
}

func (nopProcessingSupporter) Create(cfg beat.ProcessingConfig, _ bool) (beat.Processor, error) {
	return cfg.Processor, nil
}
