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
	"time"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgrpc"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/version"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/jaeger"
	"github.com/elastic/apm-server/beater/otlp"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/elasticsearch"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/sourcemap"
)

// RunServerFunc is a function which runs the APM Server until a
// fatal error occurs, or the context is cancelled.
type RunServerFunc func(context.Context, ServerParams) error

// ServerParams holds parameters for running the APM Server.
type ServerParams struct {
	// Info holds metadata about the server, such as its UUID.
	Info beat.Info

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

	// SourcemapFetcher holds a sourcemap.Fetcher, or nil if source
	// mapping is disabled.
	SourcemapFetcher sourcemap.Fetcher

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

	// NewElasticsearchClient returns an elasticsearch.Client for cfg.
	//
	// This must be used whenever an elasticsearch client might be used
	// for indexing. Under some configuration, the server will wrap the
	// client's transport such that requests will be blocked until data
	// streams have been initialised.
	NewElasticsearchClient func(cfg *elasticsearch.Config) (elasticsearch.Client, error)
}

// newBaseRunServer returns the base RunServerFunc.
func newBaseRunServer(listener net.Listener) RunServerFunc {
	return func(ctx context.Context, args ServerParams) error {
		srv, err := newServer(args, listener)
		if err != nil {
			return err
		}
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-ctx.Done():
				srv.stop()
			case <-done:
			}
		}()
		go srv.agentcfgFetchReporter.Run(ctx)
		return srv.run()
	}
}

type server struct {
	logger                *logp.Logger
	cfg                   *config.Config
	agentcfgFetchReporter agentcfg.Reporter

	httpServer *httpServer
	grpcServer *grpc.Server
}

func newServer(args ServerParams, listener net.Listener) (server, error) {
	agentcfgFetchReporter := agentcfg.NewReporter(agentcfg.NewFetcher(args.Config), args.BatchProcessor, 30*time.Second)

	ratelimitStore, err := ratelimit.NewStore(
		args.Config.AgentAuth.Anonymous.RateLimit.IPLimit,
		args.Config.AgentAuth.Anonymous.RateLimit.EventLimit,
		3, // burst multiplier
	)
	if err != nil {
		return server{}, err
	}
	authenticator, err := auth.NewAuthenticator(args.Config.AgentAuth)
	if err != nil {
		return server{}, err
	}

	// Add a model processor that rate limits, and checks authorization for the agent and service for each event.
	batchProcessor := modelprocessor.Chained{
		model.ProcessBatchFunc(rateLimitBatchProcessor),
		model.ProcessBatchFunc(authorizeEventIngestProcessor),
		args.BatchProcessor,
	}

	publishReady := func() bool {
		select {
		case <-args.PublishReady:
			return true
		default:
			return false
		}
	}

	// Create an HTTP server for serving Elastic APM agent requests.
	mux, err := api.NewMux(
		args.Info, args.Config, batchProcessor,
		authenticator, agentcfgFetchReporter, ratelimitStore,
		args.SourcemapFetcher, args.Managed, publishReady,
	)
	if err != nil {
		return server{}, err
	}
	handler := apmhttp.Wrap(mux, apmhttp.WithServerRequestIgnorer(doNotTrace), apmhttp.WithTracer(args.Tracer))
	httpServer, err := newHTTPServer(args.Logger, args.Info, args.Config, handler, listener)
	if err != nil {
		return server{}, err
	}

	// Create a gRPC server for OTLP and Jaeger.
	grpcServer, err := newGRPCServer(
		args.Logger, args.Config, args.Tracer,
		authenticator, batchProcessor, agentcfgFetchReporter, ratelimitStore,
	)
	if err != nil {
		return server{}, err
	}

	return server{
		logger:                args.Logger,
		cfg:                   args.Config,
		httpServer:            httpServer,
		grpcServer:            grpcServer,
		agentcfgFetchReporter: agentcfgFetchReporter,
	}, nil
}

func newGRPCServer(
	logger *logp.Logger,
	cfg *config.Config,
	tracer *apm.Tracer,
	authenticator *auth.Authenticator,
	batchProcessor model.BatchProcessor,
	agentcfgFetcher agentcfg.Fetcher,
	ratelimitStore *ratelimit.Store,
) (*grpc.Server, error) {
	apmInterceptor := apmgrpc.NewUnaryServerInterceptor(apmgrpc.WithRecovery(), apmgrpc.WithTracer(tracer))
	authInterceptor := interceptors.Auth(
		otlp.MethodAuthenticators(authenticator),
		jaeger.MethodAuthenticators(authenticator),
	)

	// Note that we intentionally do not use a grpc.Creds ServerOption
	// even if TLS is enabled, as TLS is handled by the net/http server.
	logger = logger.Named("grpc")
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			apmInterceptor,
			interceptors.ClientMetadata(),
			interceptors.Logging(logger),
			interceptors.Metrics(logger, otlp.RegistryMonitoringMaps, jaeger.RegistryMonitoringMaps),
			interceptors.Timeout(),
			authInterceptor,
			interceptors.AnonymousRateLimit(ratelimitStore),
		),
	)

	if cfg.AugmentEnabled {
		// Add a model processor that sets `client.ip` for events from end-user devices.
		batchProcessor = modelprocessor.Chained{
			model.ProcessBatchFunc(otlp.SetClientMetadata),
			batchProcessor,
		}
	}

	jaeger.RegisterGRPCServices(srv, logger, batchProcessor, agentcfgFetcher)
	if err := otlp.RegisterGRPCServices(srv, batchProcessor); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s server) run() error {
	s.logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	var g errgroup.Group
	g.Go(s.httpServer.start)
	g.Go(func() error {
		return s.grpcServer.Serve(s.httpServer.grpcListener)
	})
	if err := g.Wait(); err != http.ErrServerClosed {
		return err
	}
	s.logger.Infof("Server stopped")
	return nil
}

func (s server) stop() {
	s.grpcServer.GracefulStop()
	s.httpServer.stop()
}
