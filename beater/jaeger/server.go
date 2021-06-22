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

package jaeger

import (
	"context"
	"net"
	"net/http"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgrpc"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/agentcfg"
	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/interceptors"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/processor/otel"
)

// ElasticAuthTag is the name of the agent tag that will be used for auth.
// The tag value should be "Bearer <secret token" or "ApiKey <api key>".
//
// This is only relevant to the gmuxed gRPC server.
const ElasticAuthTag = "elastic-apm-auth"

// Server manages Jaeger gRPC and HTTP servers, providing methods for starting and stopping them.
//
// NOTE(axw) the standalone Jaeger gRPC and HTTP servers provided by this package are deprecated,
// and will be removed in a future release. Jaeger gRPC is now served on the primary APM Server
// port, muxed with Elastic APM HTTP traffic.
type Server struct {
	logger *logp.Logger
	grpc   struct {
		server   *grpc.Server
		listener net.Listener
	}
	http struct {
		server   *http.Server
		listener net.Listener
	}
}

// NewServer creates a new Server.
func NewServer(
	logger *logp.Logger,
	cfg *config.Config,
	tracer *apm.Tracer,
	processor model.BatchProcessor,
	fetcher agentcfg.Fetcher,
) (*Server, error) {
	if !cfg.JaegerConfig.GRPC.Enabled && !cfg.JaegerConfig.HTTP.Enabled {
		return nil, nil
	}
	traceConsumer := &otel.Consumer{Processor: processor}

	srv := &Server{logger: logger}
	if cfg.JaegerConfig.GRPC.Enabled {
		var agentAuth config.AgentAuth
		if cfg.JaegerConfig.GRPC.AuthTag != "" {
			// By default auth is not required for Jaeger - users
			// must explicitly specify which tag to use.
			agentAuth = cfg.AgentAuth
		}
		authBuilder, err := authorization.NewBuilder(agentAuth)
		if err != nil {
			return nil, err
		}

		logger = logger.Named(logs.Jaeger)
		grpcInterceptors := []grpc.UnaryServerInterceptor{
			apmgrpc.NewUnaryServerInterceptor(
				apmgrpc.WithRecovery(),
				apmgrpc.WithTracer(tracer),
			),
			interceptors.Logging(logger),
			interceptors.Metrics(logger, RegistryMonitoringMaps),
			interceptors.Timeout(),
			interceptors.Authorization(MethodAuthorizationHandlers(authBuilder, cfg.JaegerConfig.GRPC.AuthTag)),
		}

		// TODO(axw) should the listener respect cfg.MaxConnections?
		grpcListener, err := net.Listen("tcp", cfg.JaegerConfig.GRPC.Host)
		if err != nil {
			return nil, err
		}
		grpcOptions := []grpc.ServerOption{grpc.ChainUnaryInterceptor(grpcInterceptors...)}
		if cfg.JaegerConfig.GRPC.TLS != nil {
			creds := credentials.NewTLS(cfg.JaegerConfig.GRPC.TLS)
			grpcOptions = append(grpcOptions, grpc.Creds(creds))
		}
		srv.grpc.server = grpc.NewServer(grpcOptions...)
		srv.grpc.listener = grpcListener
		RegisterGRPCServices(srv.grpc.server, logger, processor, fetcher)
	}
	if cfg.JaegerConfig.HTTP.Enabled {
		// TODO(axw) should the listener respect cfg.MaxConnections?
		httpListener, err := net.Listen("tcp", cfg.JaegerConfig.HTTP.Host)
		if err != nil {
			return nil, err
		}
		httpMux, err := newHTTPMux(traceConsumer)
		if err != nil {
			return nil, err
		}
		srv.http.listener = httpListener
		srv.http.server = &http.Server{
			Handler:        apmhttp.Wrap(httpMux, apmhttp.WithTracer(tracer)),
			IdleTimeout:    cfg.IdleTimeout,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			MaxHeaderBytes: cfg.MaxHeaderSize,
		}
	}
	return srv, nil
}

// RegisterGRPCServices registers Jaeger gRPC services with srv.
func RegisterGRPCServices(
	srv *grpc.Server,
	logger *logp.Logger,
	processor model.BatchProcessor,
	fetcher agentcfg.Fetcher,
) {
	traceConsumer := &otel.Consumer{Processor: processor}
	api_v2.RegisterCollectorServiceServer(srv, &grpcCollector{traceConsumer})
	api_v2.RegisterSamplingManagerServer(srv, &grpcSampler{logger, fetcher})
}

// Serve accepts gRPC and HTTP connections, and handles Jaeger requests.
//
// Serve blocks until Stop is called, or if either of the gRPC or HTTP
// servers terminates unexpectedly.
func (s *Server) Serve() error {
	var g errgroup.Group
	if s.grpc.server != nil {
		g.Go(s.serveGRPC)
	}
	if s.http.server != nil {
		g.Go(s.serveHTTP)
	}
	return g.Wait()
}

func (s *Server) serveGRPC() error {
	s.logger.Infof("Listening for Jaeger gRPC requests on: %s", s.grpc.listener.Addr())
	return s.grpc.server.Serve(s.grpc.listener)
}

func (s *Server) serveHTTP() error {
	s.logger.Infof("Listening for Jaeger HTTP requests on: %s", s.http.listener.Addr())
	if err := s.http.server.Serve(s.http.listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop stops the gRPC and HTTP servers gracefully, causing Serve to return.
func (s *Server) Stop() {
	if s.grpc.server != nil {
		s.logger.Infof("Stopping Jaeger gRPC server")
		s.grpc.server.GracefulStop()
	}
	if s.http.server != nil {
		s.logger.Infof("Stopping Jaeger HTTP server")
		if err := s.http.server.Shutdown(context.Background()); err != nil {
			s.logger.Errorf("Error stopping Jaeger HTTP server: %s", err)
			if err := s.http.server.Close(); err != nil {
				s.logger.Errorf("Error closing Jaeger HTTP server: %s", err)
			}
		}
	}
}
