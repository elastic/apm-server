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
	"crypto/tls"
	"net/http"
	"time"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/version"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/jaeger"
	"github.com/elastic/apm-server/beater/otlp"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/publish"
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

	// SourcemapStore holds a sourcemap.Store, or nil if source
	// mapping is disabled.
	SourcemapStore *sourcemap.Store

	// BatchProcessor is the model.BatchProcessor that is used
	// for publishing events to the output, such as Elasticsearch.
	BatchProcessor model.BatchProcessor
}

type server struct {
	logger *logp.Logger
	cfg    *config.Config

	agentCtx              context.Context
	agentCancel           context.CancelFunc
	agentcfgFetchReporter agentcfg.Reporter

	httpServer   *httpServer
	grpcServer   *grpc.Server
	jaegerServer *jaeger.Server
}

func newServer(
	logger *logp.Logger,
	info beat.Info,
	cfg *config.Config,
	tracer *apm.Tracer,
	reporter publish.Reporter,
	sourcemapStore *sourcemap.Store,
	batchProcessor model.BatchProcessor,
) (*server, error) {
	agentcfgFetchReporter := agentcfg.NewReporter(agentcfg.NewFetcher(cfg), batchProcessor, 30*time.Second)
	ratelimitStore, err := ratelimit.NewStore(
		cfg.AgentAuth.Anonymous.RateLimit.IPLimit,
		cfg.AgentAuth.Anonymous.RateLimit.EventLimit,
		3, // burst multiplier
	)
	if err != nil {
		return nil, err
	}
	httpServer, err := newHTTPServer(
		logger, info, cfg, tracer, reporter, batchProcessor, agentcfgFetchReporter, ratelimitStore, sourcemapStore,
	)
	if err != nil {
		return nil, err
	}
	grpcServer, err := newGRPCServer(logger, cfg, tracer, batchProcessor, httpServer.TLSConfig, agentcfgFetchReporter, ratelimitStore)
	if err != nil {
		return nil, err
	}
	jaegerServer, err := jaeger.NewServer(logger, cfg, tracer, batchProcessor, agentcfgFetchReporter)
	if err != nil {
		return nil, err
	}
	return &server{
		logger:                logger,
		cfg:                   cfg,
		httpServer:            httpServer,
		grpcServer:            grpcServer,
		jaegerServer:          jaegerServer,
		agentcfgFetchReporter: agentcfgFetchReporter,
	}, nil
}

func newGRPCServer(
	logger *logp.Logger,
	cfg *config.Config,
	tracer *apm.Tracer,
	batchProcessor model.BatchProcessor,
	tlsConfig *tls.Config,
	agentcfgFetcher agentcfg.Fetcher,
	ratelimitStore *ratelimit.Store,
) (*grpc.Server, error) {
	// TODO(axw) share authenticator with beater/api.
	authenticator, err := auth.NewAuthenticator(cfg.AgentAuth)
	if err != nil {
		return nil, err
	}

	apmInterceptor := apmgrpc.NewUnaryServerInterceptor(apmgrpc.WithRecovery(), apmgrpc.WithTracer(tracer))
	authInterceptor := interceptors.Auth(
		otlp.MethodAuthenticators(authenticator),
		jaeger.MethodAuthenticators(authenticator, jaeger.ElasticAuthTag),
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

	// Add a model processor that sets `client.ip` for events from end-user devices.
	// TODO: Verify that updating AugmentEnabled on the cfg propagates here.
	batchProcessor = modelprocessor.Chained{
		model.ProcessBatchFunc(func(ctx context.Context, batch *model.Batch) error {
			if !cfg.AugmentEnabled {
				return nil
			}
			return otlp.SetClientMetadata(ctx, batch)
		}),
		batchProcessor,
	}

	// Add a model processor that rate limits, and checks authorization for the agent and service for each event.
	batchProcessor = modelprocessor.Chained{
		model.ProcessBatchFunc(rateLimitBatchProcessor),
		model.ProcessBatchFunc(authorizeEventIngestProcessor),
		batchProcessor,
	}

	// TODO: This needs to be updated, agentcfgFetcher can change.
	jaeger.RegisterGRPCServices(srv, logger, batchProcessor, agentcfgFetcher)
	if err := otlp.RegisterGRPCServices(srv, batchProcessor); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *server) configure(
	logger *logp.Logger,
	info beat.Info,
	cfg *config.Config,
	tracer *apm.Tracer,
	reporter publish.Reporter,
	sourcemapStore *sourcemap.Store,
	batchProcessor model.BatchProcessor,
) error {
	s.agentcfgFetchReporter = agentcfg.NewReporter(agentcfg.NewFetcher(cfg), batchProcessor, 30*time.Second)
	ratelimitStore, err := ratelimit.NewStore(
		cfg.AgentAuth.Anonymous.RateLimit.IPLimit,
		cfg.AgentAuth.Anonymous.RateLimit.EventLimit,
		3, // burst multiplier
	)
	if err != nil {
		return err
	}
	*s.cfg = *cfg
	if s.agentCancel != nil {
		s.agentCancel()
		s.startFetchReporter(s.agentCtx)
	}
	return s.httpServer.configure(
		logger,
		info,
		cfg,
		tracer,
		reporter,
		batchProcessor,
		s.agentcfgFetchReporter,
		ratelimitStore,
		sourcemapStore,
	)
}

func (s *server) startFetchReporter(ctx context.Context) {
	s.agentCtx = ctx
	ctx, s.agentCancel = context.WithCancel(s.agentCtx)
	go s.agentcfgFetchReporter.Run(ctx)
}

func (s *server) run(ctx context.Context, _ ServerParams) error {
	done := make(chan struct{})
	defer func() {
		defer close(done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			s.stop()
		case <-done:
		}
	}()
	s.startFetchReporter(ctx)

	s.logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	var g errgroup.Group
	g.Go(s.httpServer.start)
	g.Go(func() error {
		return s.grpcServer.Serve(s.httpServer.grpcListener)
	})
	if s.jaegerServer != nil {
		g.Go(s.jaegerServer.Serve)
	}
	if err := g.Wait(); err != http.ErrServerClosed {
		return err
	}
	s.logger.Infof("Server stopped")
	return nil
}

func (s server) stop() {
	if s.jaegerServer != nil {
		s.jaegerServer.Stop()
	}
	s.grpcServer.GracefulStop()
	s.httpServer.stop()
	if s.agentCancel != nil {
		s.agentCancel()
	}
}
