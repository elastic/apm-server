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
	"github.com/elastic/apm-server/beater/jaeger"
	"github.com/elastic/apm-server/beater/otlp"
	"github.com/elastic/apm-server/kibana"
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

	// Reporter is the publish.Reporter that the APM Server
	// should use for reporting events.
	Reporter publish.Reporter
}

// runServer runs the APM Server until a fatal error occurs, or ctx is cancelled.
func runServer(ctx context.Context, args ServerParams) error {
	srv, err := newServer(args.Logger, args.Config, args.Tracer, args.Reporter)
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
	return srv.run()
}

type server struct {
	logger *logp.Logger
	cfg    *config.Config

	httpServer   *httpServer
	grpcServer   *grpc.Server
	jaegerServer *jaeger.Server
	reporter     publish.Reporter
}

func newServer(logger *logp.Logger, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter) (server, error) {
	httpServer, err := newHTTPServer(logger, cfg, tracer, reporter)
	if err != nil {
		return server{}, err
	}
	grpcServer, err := newGRPCServer(logger, cfg, tracer, reporter, httpServer.TLSConfig)
	if err != nil {
		return server{}, err
	}
	jaegerServer, err := jaeger.NewServer(logger, cfg, tracer, reporter)
	if err != nil {
		return server{}, err
	}
	return server{
		logger:       logger,
		cfg:          cfg,
		httpServer:   httpServer,
		grpcServer:   grpcServer,
		jaegerServer: jaegerServer,
		reporter:     reporter,
	}, nil
}

func newGRPCServer(
	logger *logp.Logger, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter, tlsConfig *tls.Config,
) (*grpc.Server, error) {
	// TODO(axw) share auth builder with beater/api.
	authBuilder, err := authorization.NewBuilder(cfg)
	if err != nil {
		return nil, err
	}

	// NOTE(axw) even if TLS is enabled we should not use grpc.Creds, as TLS is handled by the net/http server.
	apmInterceptor := apmgrpc.NewUnaryServerInterceptor(apmgrpc.WithRecovery(), apmgrpc.WithTracer(tracer))
	authInterceptor := newAuthUnaryServerInterceptor(authBuilder)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(apmInterceptor, authInterceptor))

	var kibanaClient kibana.Client
	var agentcfgFetcher *agentcfg.Fetcher
	if cfg.Kibana.Enabled {
		kibanaClient = kibana.NewConnectingClient(&cfg.Kibana.ClientConfig)
		agentcfgFetcher = agentcfg.NewFetcher(kibanaClient, cfg.AgentConfig.Cache.Expiration)
	}
	jaeger.RegisterGRPCServices(srv, authBuilder, jaeger.ElasticAuthTag, logger, reporter, kibanaClient, agentcfgFetcher)
	if err := otlp.RegisterGRPCServices(srv, reporter, logger); err != nil {
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
