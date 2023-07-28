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
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.elastic.co/apm/module/apmgrpc/v2"
	"go.elastic.co/apm/module/apmotel/v2"
	"go.elastic.co/apm/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"

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
	"github.com/elastic/go-docappender"
	"github.com/elastic/go-ucfg"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/interceptors"
	javaattacher "github.com/elastic/apm-server/internal/beater/java_attacher"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/idxmgmt"
	"github.com/elastic/apm-server/internal/kibana"
	srvmodelprocessor "github.com/elastic/apm-server/internal/model/modelprocessor"
	"github.com/elastic/apm-server/internal/publish"
	"github.com/elastic/apm-server/internal/sourcemap"
	"github.com/elastic/apm-server/internal/version"
)

var (
	monitoringRegistry         = monitoring.Default.NewRegistry("apm-server.sampling")
	transactionsDroppedCounter = monitoring.NewInt(monitoringRegistry, "transactions_dropped")
)

// Runner initialises and runs and orchestrates the APM Server
// HTTP and gRPC servers, event processing pipeline, and output.
type Runner struct {
	wrapServer WrapServerFunc
	logger     *logp.Logger
	rawConfig  *agentconfig.C

	config                    *config.Config
	fleetConfig               *config.Fleet
	outputConfig              agentconfig.Namespace
	elasticsearchOutputConfig *agentconfig.C

	listener net.Listener
}

// RunnerParams holds parameters for NewRunner.
type RunnerParams struct {
	// Config holds the full, raw, configuration, including apm-server.*
	// and output.* attributes.
	Config *agentconfig.C

	// Logger holds a logger to use for logging throughout the APM Server.
	Logger *logp.Logger

	// WrapServer holds an optional WrapServerFunc, for wrapping the
	// ServerParams and RunServerFunc used to run the APM Server.
	//
	// If WrapServer is nil, no wrapping will occur.
	WrapServer WrapServerFunc
}

// NewRunner returns a new Runner that runs APM Server with the given parameters.
func NewRunner(args RunnerParams) (*Runner, error) {
	var unpackedConfig struct {
		APMServer  *agentconfig.C        `config:"apm-server"`
		Output     agentconfig.Namespace `config:"output"`
		Fleet      *config.Fleet         `config:"fleet"`
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
	cfg, err := config.NewConfig(unpackedConfig.APMServer, elasticsearchOutputConfig)
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
		fleetConfig:               unpackedConfig.Fleet,
		outputConfig:              unpackedConfig.Output,
		elasticsearchOutputConfig: elasticsearchOutputConfig,

		listener: listener,
	}, nil
}

// Run runs the server, blocking until ctx is cancelled.
func (s *Runner) Run(ctx context.Context) error {
	defer s.listener.Close()
	g, ctx := errgroup.WithContext(ctx)

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

	// Obtain the memory limit for the APM Server process. Certain config
	// values will be sized according to the maximum memory set for the server.
	var memLimitGB float64
	if cgroupReader := newCgroupReader(); cgroupReader != nil {
		if limit, err := cgroupMemoryLimit(cgroupReader); err != nil {
			s.logger.Warn(err)
		} else {
			memLimitGB = float64(limit) / 1024 / 1024 / 1024
		}
	}
	if limit, err := systemMemoryLimit(); err != nil {
		s.logger.Warn(err)
	} else {
		var fallback bool
		if memLimitGB <= 0 {
			s.logger.Info("no cgroups detected, falling back to total system memory")
			fallback = true
		}
		if memLimitGB > float64(limit) {
			s.logger.Info("cgroup memory limit exceed available memory, falling back to the total system memory")
			fallback = true
		}
		if fallback {
			// If no cgroup limit is set, return a fraction of the total memory
			// to have a margin of safety for other processes. The fraction value
			// of 0.625 is used to keep the 80% of the total system memory limit
			// to be 50% of the total for calculating the number of decoders.
			memLimitGB = float64(limit) / 1024 / 1024 / 1024 * 0.625
		}
	}
	if memLimitGB <= 0 {
		memLimitGB = 1
		s.logger.Infof(
			"failed to discover memory limit, default to %0.1fgb of memory",
			memLimitGB,
		)
	}
	if s.config.MaxConcurrentDecoders == 0 {
		s.config.MaxConcurrentDecoders = maxConcurrentDecoders(memLimitGB)
		s.logger.Infof("MaxConcurrentDecoders set to %d based on 80 percent of %0.1fgb of memory",
			s.config.MaxConcurrentDecoders, memLimitGB,
		)
	}
	if s.config.Aggregation.Transactions.MaxTransactionGroups <= 0 {
		s.config.Aggregation.Transactions.MaxTransactionGroups = maxTxGroupsForAggregation(memLimitGB)
		s.logger.Infof("Transactions.MaxTransactionGroups set to %d based on %0.1fgb of memory",
			s.config.Aggregation.Transactions.MaxTransactionGroups, memLimitGB,
		)
	}
	if s.config.Aggregation.Transactions.MaxServices <= 0 {
		s.config.Aggregation.Transactions.MaxServices = maxGroupsForAggregation(memLimitGB)
		s.logger.Infof("Transactions.MaxServices set to %d based on %0.1fgb of memory",
			s.config.Aggregation.Transactions.MaxServices, memLimitGB,
		)
	}
	if s.config.Aggregation.ServiceTransactions.MaxGroups <= 0 {
		s.config.Aggregation.ServiceTransactions.MaxGroups = maxGroupsForAggregation(memLimitGB)
		s.logger.Infof("ServiceTransactions.MaxGroups for service aggregation set to %d based on %0.1fgb of memory",
			s.config.Aggregation.ServiceTransactions.MaxGroups, memLimitGB,
		)
	}

	// Send config to telemetry.
	recordAPMServerConfig(s.config)

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

	if s.config.JavaAttacherConfig.Enabled {
		if !inElasticCloud {
			go func() {
				attacher, err := javaattacher.New(s.config.JavaAttacherConfig)
				if err != nil {
					s.logger.Errorf("failed to start java attacher: %v", err)
					return
				}
				if err := attacher.Run(ctx); err != nil {
					s.logger.Errorf("failed to run java attacher: %v", err)
				}
			}()
		} else {
			s.logger.Error("java attacher not supported in cloud environments")
		}
	}

	instrumentation, err := instrumentation.New(s.rawConfig, "apm-server", version.Version)
	if err != nil {
		return err
	}
	tracer := instrumentation.Tracer()
	tracerServerListener := instrumentation.Listener()
	if tracerServerListener != nil {
		defer tracerServerListener.Close()
	}
	defer tracer.Close()

	provider, err := apmotel.NewTracerProvider(apmotel.WithAPMTracer(tracer))
	if err != nil {
		return err
	}
	otel.SetTracerProvider(provider)

	exporter, err := apmotel.NewGatherer()
	if err != nil {
		return err
	}
	mp := metric.NewMeterProvider(metric.WithReader(exporter))
	otel.SetMeterProvider(mp)
	tracer.RegisterMetricsGatherer(exporter)

	// Ensure the libbeat output and go-elasticsearch clients do not index
	// any events to Elasticsearch before the integration is ready.
	publishReady := make(chan struct{})
	drain := make(chan struct{})
	g.Go(func() error {
		if err := s.waitReady(ctx, kibanaClient, tracer); err != nil {
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
	newElasticsearchClient := func(cfg *elasticsearch.Config) (*elasticsearch.Client, error) {
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
		fetcher, cancel, err := newSourcemapFetcher(
			s.config.RumConfig.SourceMapping,
			kibanaClient, newElasticsearchClient,
			tracer,
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
		apmgrpc.NewUnaryServerInterceptor(apmgrpc.WithRecovery(), apmgrpc.WithTracer(tracer)),
		interceptors.ClientMetadata(),
		interceptors.Logging(gRPCLogger),
		interceptors.Metrics(gRPCLogger),
		interceptors.Timeout(),
		interceptors.Auth(authenticator),
		interceptors.AnonymousRateLimit(ratelimitStore),
	))

	// Create the BatchProcessor chain that is used to process all events,
	// including the metrics aggregated by APM Server.
	finalBatchProcessor, closeFinalBatchProcessor, err := s.newFinalBatchProcessor(
		tracer, newElasticsearchClient, memLimitGB,
	)
	if err != nil {
		return err
	}
	batchProcessor := srvmodelprocessor.NewTracer("beater.ProcessBatch", modelprocessor.Chained{
		// Ensure all events have observer.*, ecs.*, and data_stream.* fields added,
		// and are counted in metrics. This is done in the final processors to ensure
		// aggregated metrics are also processed.
		newObserverBatchProcessor(),
		&modelprocessor.SetDataStream{Namespace: s.config.DataStreams.Namespace},
		srvmodelprocessor.NewEventCounter(monitoring.Default.GetRegistry("apm-server")),

		// The server always drops non-RUM unsampled transactions. We store RUM unsampled
		// transactions as they are needed by the User Experience app, which performs
		// aggregations over dimensions that are not available in transaction metrics.
		//
		// It is important that this is done just before calling the publisher to
		// avoid affecting aggregations.
		modelprocessor.NewDropUnsampled(false /* don't drop RUM unsampled transactions*/, func(i int64) {
			transactionsDroppedCounter.Add(i)
		}),
		finalBatchProcessor,
	})

	agentConfigFetcher, fetcherRunFunc, err := newAgentConfigFetcher(
		ctx,
		s.config,
		kibanaClient,
		newElasticsearchClient,
		tracer,
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
		Tracer:                 tracer,
		Authenticator:          authenticator,
		RateLimitStore:         ratelimitStore,
		BatchProcessor:         batchProcessor,
		AgentConfig:            agentConfigReporter,
		SourcemapFetcher:       sourcemapFetcher,
		PublishReady:           publishReady,
		KibanaClient:           kibanaClient,
		NewElasticsearchClient: newElasticsearchClient,
		GRPCServer:             grpcServer,
		Semaphore:              semaphore.NewWeighted(int64(s.config.MaxConcurrentDecoders)),
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
		modelpb.ProcessBatchFunc(removeEventReceivedBatchProcessor),

		// Pre-process events before they are sent to the final processors for
		// aggregation, sampling, and indexing.
		modelprocessor.SetHostHostname{},
		modelprocessor.SetServiceNodeName{},
		modelprocessor.SetGroupingKey{},
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
	if tracerServerListener != nil {
		tracerServer, err := newTracerServer(s.config, tracerServerListener, s.logger, serverParams.BatchProcessor, serverParams.Semaphore)
		if err != nil {
			return fmt.Errorf("failed to create self-instrumentation server: %w", err)
		}
		g.Go(func() error {
			if err := tracerServer.Serve(tracerServerListener); err != http.ErrServerClosed {
				return err
			}
			return nil
		})
		go func() {
			<-ctx.Done()
			tracerServer.Shutdown(backgroundContext)
		}()
	}

	result := g.Wait()
	if err := closeFinalBatchProcessor(backgroundContext); err != nil {
		result = multierror.Append(result, err)
	}
	return result
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

// maxGroupsForAggregation calculates the maximum service groups that a
// particular memory limit can have. This will be scaled linearly for bigger
// instances.
func maxGroupsForAggregation(memLimitGB float64) int {
	const maxMemGB = 64
	if memLimitGB > maxMemGB {
		memLimitGB = maxMemGB
	}
	return int(memLimitGB * 1_000)
}

// maxTxGroupsForAggregation calculates the maximum transaction groups that a
// particular memory limit can have. This will be scaled linearly for bigger
// instances.
func maxTxGroupsForAggregation(memLimitGB float64) int {
	const maxMemGB = 64
	if memLimitGB > maxMemGB {
		memLimitGB = maxMemGB
	}
	return int(memLimitGB * 5_000)
}

// waitReady waits until the server is ready to index events.
func (s *Runner) waitReady(
	ctx context.Context,
	kibanaClient *kibana.Client,
	tracer *apm.Tracer,
) error {
	var preconditions []func(context.Context) error
	var esOutputClient *elasticsearch.Client
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
				license, err := getElasticsearchLicense(ctx, esOutputClient)
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
	fleetManaged := s.fleetConfig != nil
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
	return waitReady(ctx, s.config.WaitReadyInterval, tracer, s.logger, check)
}

// newFinalBatchProcessor returns the final model.BatchProcessor that publishes events,
// and a cleanup function which should be called on server shutdown. If the output is
// "elasticsearch", then we use docappender; otherwise we use the libbeat publisher.
func (s *Runner) newFinalBatchProcessor(
	tracer *apm.Tracer,
	newElasticsearchClient func(cfg *elasticsearch.Config) (*elasticsearch.Client, error),
	memLimit float64,
) (modelpb.BatchProcessor, func(context.Context) error, error) {

	monitoring.Default.Remove("libbeat")
	libbeatMonitoringRegistry := monitoring.Default.NewRegistry("libbeat")
	if s.elasticsearchOutputConfig == nil {
		return s.newLibbeatFinalBatchProcessor(tracer, libbeatMonitoringRegistry)
	}

	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	outputRegistry := stateRegistry.GetRegistry("output")
	if outputRegistry != nil {
		outputRegistry.Clear()
	} else {
		outputRegistry = stateRegistry.NewRegistry("output")
	}
	monitoring.NewString(outputRegistry, "name").Set("elasticsearch")

	var esConfig struct {
		*elasticsearch.Config `config:",inline"`
		FlushBytes            string        `config:"flush_bytes"`
		FlushInterval         time.Duration `config:"flush_interval"`
		MaxRequests           int           `config:"max_requests"`
		Scaling               struct {
			Enabled *bool `config:"enabled"`
		} `config:"autoscaling"`
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
	var scalingCfg docappender.ScalingConfig
	if enabled := esConfig.Scaling.Enabled; enabled != nil {
		scalingCfg.Disabled = !*enabled
	}
	opts := docappender.Config{
		CompressionLevel: esConfig.CompressionLevel,
		FlushBytes:       flushBytes,
		FlushInterval:    esConfig.FlushInterval,
		Tracer:           tracer,
		MaxRequests:      esConfig.MaxRequests,
		Scaling:          scalingCfg,
		Logger:           zap.New(s.logger.Core(), zap.WithCaller(true)),
	}
	opts = docappenderConfig(opts, memLimit, s.logger)
	appender, err := docappender.New(client, opts)
	if err != nil {
		return nil, nil, err
	}

	// Install our own libbeat-compatible metrics callback which uses the docappender stats.
	// All the metrics below are required to be reported to be able to display all relevant
	// fields in the Stack Monitoring UI.
	monitoring.NewFunc(libbeatMonitoringRegistry, "output.write", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		v.OnKey("bytes")
		v.OnInt(appender.Stats().BytesTotal)
	})
	outputType := monitoring.NewString(libbeatMonitoringRegistry.GetRegistry("output"), "type")
	outputType.Set("elasticsearch")
	monitoring.NewFunc(libbeatMonitoringRegistry, "output.events", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		stats := appender.Stats()
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
	monitoring.NewFunc(libbeatMonitoringRegistry, "pipeline.events", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		v.OnKey("total")
		v.OnInt(appender.Stats().Added)
	})
	monitoring.Default.Remove("output")
	monitoring.NewFunc(monitoring.Default, "output.elasticsearch.bulk_requests", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		stats := appender.Stats()
		v.OnKey("available")
		v.OnInt(stats.AvailableBulkRequests)
		v.OnKey("completed")
		v.OnInt(stats.BulkRequests)
	})
	monitoring.NewFunc(monitoring.Default, "output.elasticsearch.indexers", func(_ monitoring.Mode, v monitoring.Visitor) {
		v.OnRegistryStart()
		defer v.OnRegistryFinished()
		stats := appender.Stats()
		v.OnKey("active")
		v.OnInt(stats.IndexersActive)
		v.OnKey("created")
		v.OnInt(stats.IndexersCreated)
		v.OnKey("destroyed")
		v.OnInt(stats.IndexersDestroyed)
	})
	return newDocappenderBatchProcessor(appender), appender.Close, nil
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
		return opts
	}
	// This formula yields the following max requests for APM Server sized:
	// 1	2 	4	8	15	30
	// 10	12	14	19	28	46
	maxRequests := int(float64(10) + memLimit*1.5)
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
) (modelpb.BatchProcessor, func(context.Context) error, error) {
	// When the publisher stops cleanly it will close its pipeline client,
	// calling the acker's Close method and unblock Wait.
	acker := publish.NewWaitPublishedAcker()
	acker.Open()

	hostname, _ := os.Hostname()
	beatInfo := beat.Info{
		Beat:        "apm-server",
		IndexPrefix: "apm-server",
		Version:     version.Version,
		Hostname:    hostname,
		Name:        hostname,
	}

	stateRegistry := monitoring.GetNamespace("state").GetRegistry()
	stateRegistry.Remove("queue")
	monitors := pipeline.Monitors{
		Metrics:   libbeatMonitoringRegistry,
		Telemetry: stateRegistry,
		Logger:    logp.L().Named("publisher"),
		Tracer:    tracer,
	}
	outputFactory := func(stats outputs.Observer) (string, outputs.Group, error) {
		if !s.outputConfig.IsSet() {
			return "", outputs.Group{}, nil
		}
		indexSupporter := idxmgmt.NewSupporter(nil, s.rawConfig)
		outputName := s.outputConfig.Name()
		output, err := outputs.Load(indexSupporter, beatInfo, stats, outputName, s.outputConfig.Config())
		return outputName, output, err
	}
	pipeline, err := pipeline.Load(beatInfo, monitors, pipeline.Config{}, nopProcessingSupporter{}, outputFactory)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create libbeat output pipeline: %w", err)
	}
	pipelineConnector := pipetool.WithACKer(pipeline, acker)
	publisher, err := publish.NewPublisher(pipelineConnector, tracer)
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
	newElasticsearchClient func(*elasticsearch.Config) (*elasticsearch.Client, error),
	tracer *apm.Tracer,
) (sourcemap.Fetcher, context.CancelFunc, error) {
	esClient, err := newElasticsearchClient(cfg.ESConfig)
	if err != nil {
		return nil, nil, err
	}

	var fetchers []sourcemap.Fetcher

	// start background sync job
	ctx, cancel := context.WithCancel(context.Background())
	metadataFetcher, invalidationChan := sourcemap.NewMetadataFetcher(ctx, esClient, sourcemapIndex, tracer)

	esFetcher := sourcemap.NewElasticsearchFetcher(esClient, sourcemapIndex)
	size := 128
	cachingFetcher, err := sourcemap.NewBodyCachingFetcher(esFetcher, size, invalidationChan)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	sourcemapFetcher := sourcemap.NewSourcemapFetcher(metadataFetcher, cachingFetcher)

	fetchers = append(fetchers, sourcemapFetcher)

	if kibanaClient != nil {
		fetchers = append(fetchers, sourcemap.NewKibanaFetcher(kibanaClient))
	}

	chained := sourcemap.NewChainedFetcher(fetchers)

	return chained, cancel, nil
}

// TODO: This is copying behavior from libbeat:
// https://github.com/elastic/beats/blob/b9ced47dba8bb55faa3b2b834fd6529d3c4d0919/libbeat/cmd/instance/beat.go#L927-L950
// Remove this when cluster_uuid no longer needs to be queried from ES.
func queryClusterUUID(ctx context.Context, esClient *elasticsearch.Client) error {
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
