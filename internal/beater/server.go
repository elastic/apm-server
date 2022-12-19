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
	"net/http"

	"go.elastic.co/apm/module/apmgorilla/v2"
	"go.elastic.co/apm/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/elastic/beats/v7/libbeat/version"
	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-data/model"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/api"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/jaeger"
	"github.com/elastic/apm-server/internal/beater/otlp"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
	"github.com/elastic/apm-server/internal/elasticsearch"
	"github.com/elastic/apm-server/internal/kibana"
	"github.com/elastic/apm-server/internal/model/modelprocessor"
	"github.com/elastic/apm-server/internal/sourcemap"
)

// WrapServerFunc is a function for injecting behaviour into ServerParams
// and RunServerFunc.
//
// WrapServerFunc may modify ServerParams, for example by wrapping the
// BatchProcessor with additional processors. Similarly, WrapServerFunc
// may wrap the RunServerFunc to run additional goroutines along with the
// server.
//
// WrapServerFunc may keep a reference to the provided ServerParams's
// BatchProcessor for asynchronous event publication, such as for
// aggregated metrics. All other events (i.e. those decoded from
// agent payloads) should be sent to the BatchProcessor in the
// ServerParams provided to RunServerFunc; this BatchProcessor will
// have rate-limiting, authorization, and data preprocessing applied.
type WrapServerFunc func(ServerParams, RunServerFunc) (ServerParams, RunServerFunc, error)

// RunServerFunc is a function which runs the APM Server until a
// fatal error occurs, or the context is cancelled.
type RunServerFunc func(context.Context, ServerParams) error

// ServerParams holds parameters for running the APM Server.
type ServerParams struct {
	// Config is the configuration used for running the APM Server.
	Config *config.Config

	// Managed indicates that the server is managed by Fleet.
	Managed bool

	// Namespace holds the data stream namespace for the server.
	Namespace string

	// Logger is the logger for the beater component.
	Logger *logp.Logger

	// Tracer is an apm.Tracer that the APM Server may use
	// for self-instrumentation.
	Tracer *apm.Tracer

	// Authenticator holds an authenticator that can be used for
	// authenticating clients, and obtaining authentication details
	// and an auth.Authorizer for authorizing the client for future
	// actions on resources.
	Authenticator *auth.Authenticator

	// RateLimitStore holds an IP-based rate-limiter LRU cache.
	RateLimitStore *ratelimit.Store

	// SourcemapFetcher holds a sourcemap.Fetcher, or nil if source
	// mapping is disabled.
	SourcemapFetcher sourcemap.Fetcher

	// AgentConfig holds an interface for fetching agent configuration.
	AgentConfig agentcfg.Fetcher

	// BatchProcessor is the model.BatchProcessor that is used
	// for publishing events to the output, such as Elasticsearch.
	BatchProcessor model.BatchProcessor

	// PublishReady holds a channel which will be signalled when the serve
	// is ready to publish events. Readiness means that preconditions for
	// event publication have been met, including icense checks for some
	// features and waiting for the Fleet integration to be installed
	// when running in standalone mode.
	//
	// Even if the server is not ready to publish events, it will still
	// accept events and enqueue them for later publication.
	PublishReady <-chan struct{}

	// KibanaClient holds a Kibana client if the server has Kibana
	// configuration. If the server has no Kibana configuration, this
	// field will be nil.
	KibanaClient *kibana.Client

	// NewElasticsearchClient returns an elasticsearch.Client for cfg.
	//
	// This must be used whenever an elasticsearch client might be used
	// for indexing. Under some configuration, the server will wrap the
	// client's transport such that requests will be blocked until data
	// streams have been initialised.
	NewElasticsearchClient func(cfg *elasticsearch.Config) (*elasticsearch.Client, error)

	// GRPCServer holds a *grpc.Server to which services will be registered
	// for receiving data, configuration requests, etc.
	//
	// The gRPC server is configured with various interceptors, including
	// authentication/authorization, logging, metrics, and tracing.
	// See package internal/beater/interceptors for details.
	GRPCServer *grpc.Server
}

// newBaseRunServer returns the base RunServerFunc.
func newBaseRunServer(listener net.Listener) RunServerFunc {
	return func(ctx context.Context, args ServerParams) error {
		srv, err := newServer(args, listener)
		if err != nil {
			return err
		}
		return srv.run(ctx)
	}
}

type server struct {
	logger *logp.Logger
	cfg    *config.Config

	httpServer *httpServer
	grpcServer *grpc.Server
}

func newServer(args ServerParams, listener net.Listener) (server, error) {
	publishReady := func() bool {
		select {
		case <-args.PublishReady:
			return true
		default:
			return false
		}
	}

	// Create an HTTP server for serving Elastic APM agent requests.
	router, err := api.NewMux(
		args.Config, args.BatchProcessor,
		args.Authenticator, args.AgentConfig, args.RateLimitStore,
		args.SourcemapFetcher, args.Managed, publishReady,
	)
	if err != nil {
		return server{}, err
	}
	apmgorilla.Instrument(router, apmgorilla.WithRequestIgnorer(doNotTrace), apmgorilla.WithTracer(args.Tracer))
	httpServer, err := newHTTPServer(args.Logger, args.Config, router, listener)
	if err != nil {
		return server{}, err
	}

	otlpBatchProcessor := args.BatchProcessor
	if args.Config.AugmentEnabled {
		// Add a model processor that sets `client.ip` for events from end-user devices.
		otlpBatchProcessor = modelprocessor.Chained{
			model.ProcessBatchFunc(otlp.SetClientMetadata),
			otlpBatchProcessor,
		}
	}
	zapLogger := zap.New(args.Logger.Core(), zap.WithCaller(true))
	otlp.RegisterGRPCServices(args.GRPCServer, zapLogger, otlpBatchProcessor)
	jaeger.RegisterGRPCServices(args.GRPCServer, zapLogger, args.BatchProcessor, args.AgentConfig)

	return server{
		logger:     args.Logger,
		cfg:        args.Config,
		httpServer: httpServer,
		grpcServer: args.GRPCServer,
	}, nil
}

func (s server) run(ctx context.Context) error {
	s.logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	defer s.logger.Infof("Server stopped")

	g, ctx := errgroup.WithContext(ctx)
	g.Go(s.httpServer.start)
	g.Go(func() error {
		return s.grpcServer.Serve(s.httpServer.grpcListener)
	})
	g.Go(func() error {
		<-ctx.Done()
		s.grpcServer.GracefulStop()
		s.httpServer.stop()
		return nil
	})
	if err := g.Wait(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func newAgentConfigFetcher(cfg *config.Config, kibanaClient *kibana.Client) agentcfg.Fetcher {
	if cfg.AgentConfigs != nil || kibanaClient == nil {
		// Direct agent configuration is present, disable communication with kibana.
		agentConfigurations := make([]agentcfg.AgentConfig, len(cfg.AgentConfigs))
		for i, in := range cfg.AgentConfigs {
			agentConfigurations[i] = agentcfg.AgentConfig{
				ServiceName:        in.Service.Name,
				ServiceEnvironment: in.Service.Environment,
				AgentName:          in.AgentName,
				Etag:               in.Etag,
				Config:             in.Config,
			}
		}
		return agentcfg.NewDirectFetcher(agentConfigurations)
	}
	return agentcfg.NewKibanaFetcher(kibanaClient, cfg.KibanaAgentConfig.Cache.Expiration)
}
