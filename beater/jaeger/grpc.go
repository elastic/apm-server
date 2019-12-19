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
	"net"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/pkg/errors"
	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/beater/api/jaeger"
	"github.com/elastic/apm-server/beater/config"
	processor "github.com/elastic/apm-server/processor/otel"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

const (
	networkTCP = "tcp"
)

// GRPCServer allows to start and stop a Jaeger gRPC Server with a Jaeger Collector
type GRPCServer struct {
	logger     *logp.Logger
	grpcServer *grpc.Server
	host       string
	collector  jaeger.GRPCCollector
}

// NewGRPCServer creates instance of Jaeger GRPCServer
func NewGRPCServer(logger *logp.Logger, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter) (*GRPCServer, error) {
	if !cfg.JaegerConfig.Enabled {
		return nil, nil
	}

	grpcOptions := []grpc.ServerOption{grpc.UnaryInterceptor(apmgrpc.NewUnaryServerInterceptor(
		apmgrpc.WithRecovery(),
		apmgrpc.WithTracer(tracer)))}
	if cfg.JaegerConfig.GRPC.TLS != nil {
		creds := credentials.NewTLS(cfg.JaegerConfig.GRPC.TLS)
		grpcOptions = append(grpcOptions, grpc.Creds(creds))
	}
	grpcServer := grpc.NewServer(grpcOptions...)
	consumer := &processor.Consumer{
		Reporter:        reporter,
		TransformConfig: transform.Config{},
	}

	return &GRPCServer{
		logger:     logger,
		collector:  jaeger.NewGRPCCollector(consumer),
		grpcServer: grpcServer,
		host:       cfg.JaegerConfig.GRPC.Host,
	}, nil
}

// Start gRPC server to listen for incoming Jaeger trace requests.
//TODO(simi) to add support for sampling: api_v2.RegisterSamplingManagerServer
func (jc *GRPCServer) Start() error {
	jc.logger.Infof("Starting Jaeger collector listening on: %s", jc.host)

	api_v2.RegisterCollectorServiceServer(jc.grpcServer, jc.collector)

	listener, err := net.Listen(networkTCP, jc.host)
	if err != nil {
		return errors.Wrapf(err, "error starting Jaeger collector listening on: %s", jc.host)
	}
	return jc.grpcServer.Serve(listener)
}

// Stop gRPC server gracefully.
func (jc *GRPCServer) Stop() {
	jc.logger.Infof("Stopping Jaeger collector listening on: %s", jc.host)
	jc.grpcServer.GracefulStop()
}
