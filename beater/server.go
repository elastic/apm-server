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
	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/interceptors"
	"github.com/elastic/apm-server/beater/jaeger"
	"github.com/elastic/apm-server/beater/otlp"
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modelprocessor"
	"github.com/elastic/apm-server/publish"
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

	// BatchProcessor is the model.BatchProcessor that is used
	// for publishing events to the output, such as Elasticsearch.
	BatchProcessor model.BatchProcessor
}

// newBaseRunServer returns the base RunServerFunc.
//
// reporter is the publish.Reporter that the server should use
// uploading sourcemaps and publishing its onboarding doc.
// Everything else should be using ServerParams.BatchProcessor.
//
// Once we remove sourcemap uploading and onboarding docs, we
// should remove the reporter parameter.
func newBaseRunServer(reporter publish.Reporter) RunServerFunc {
	return func(ctx context.Context, args ServerParams) error {
		srv, err := newServer(args.Logger, args.Info, args.Config, args.Tracer, reporter, args.BatchProcessor)
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
	batchProcessor model.BatchProcessor,
) (server, error) {
	agentcfgFetchReporter := agentcfg.NewReporter(agentcfg.NewFetcher(cfg), batchProcessor, 30*time.Second)
	ratelimitStore, err := ratelimit.NewStore(
		cfg.RumConfig.EventRate.LruSize,
		cfg.RumConfig.EventRate.Limit,
		3, // burst multiplier
	)
	if err != nil {
		return server{}, err
	}
	httpServer, err := newHTTPServer(logger, info, cfg, tracer, reporter, batchProcessor, agentcfgFetchReporter, ratelimitStore)
	if err != nil {
		return server{}, err
	}
	grpcServer, err := newGRPCServer(logger, cfg, tracer, batchProcessor, httpServer.TLSConfig, agentcfgFetchReporter, ratelimitStore)
	if err != nil {
		return server{}, err
	}
	jaegerServer, err := jaeger.NewServer(logger, cfg, tracer, batchProcessor, agentcfgFetchReporter)
	if err != nil {
		return server{}, err
	}
	return server{
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
	// TODO(axw) share auth builder with beater/api.
	authBuilder, err := authorization.NewBuilder(cfg.AgentAuth)
	if err != nil {
		return nil, err
	}

	apmInterceptor := apmgrpc.NewUnaryServerInterceptor(apmgrpc.WithRecovery(), apmgrpc.WithTracer(tracer))
	authInterceptor := interceptors.Authorization(
		otlp.MethodAuthorizationHandlers(authBuilder),
		jaeger.MethodAuthorizationHandlers(authBuilder, jaeger.ElasticAuthTag),
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
			modelprocessor.MetadataProcessorFunc(otlp.SetClientMetadata),
			batchProcessor,
		}
	}

	// Add a model processor that rate limits, and checks authorization for the agent and service for each event.
	batchProcessor = modelprocessor.Chained{
		model.ProcessBatchFunc(rateLimitBatchProcessor),
		modelprocessor.MetadataProcessorFunc(verifyAuthorizedFor),
		batchProcessor,
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
}
