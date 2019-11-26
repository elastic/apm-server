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

	"github.com/soheilhy/cmux"

	"golang.org/x/sync/errgroup"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/net/netutil"
	"google.golang.org/grpc"

	"github.com/elastic/beats/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/version"
	"github.com/open-telemetry/opentelemetry-collector/observability"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
)

type server struct {
	http *http.Server
	grpc *grpc.Server
}

func newServer(cfg *config.Config, tracer *apm.Tracer, report publish.Reporter) (*server, error) {
	mux, err := api.NewMux(cfg, report)
	if err != nil {
		return nil, err
	}

	httpServer := &http.Server{
		Addr: cfg.Host,
		Handler: apmhttp.Wrap(mux,
			apmhttp.WithServerRequestIgnorer(doNotTrace),
			apmhttp.WithTracer(tracer),
		),
		IdleTimeout:    cfg.IdleTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderSize,
	}

	if cfg.TLS.IsEnabled() {
		tlsServerConfig, err := tlscommon.LoadTLSServerConfig(cfg.TLS)
		if err != nil {
			return nil, err
		}
		httpServer.TLSConfig = tlsServerConfig.BuildModuleConfig(cfg.Host)
	}
	grpcServer := observability.GRPCServerWithObservabilityEnabled()
	api.RegisterGRPC(cfg, report, grpcServer)

	return &server{http: httpServer, grpc: grpcServer}, nil
}

func doNotTrace(req *http.Request) bool {
	if req.RemoteAddr == "pipe" {
		// Don't trace requests coming from self,
		// or we will go into a continuous cycle.
		return true
	}
	if req.URL.Path == api.RootPath {
		// Don't trace root url (healthcheck) requests.
		return true
	}
	return false
}

func run(logger *logp.Logger, server *server, lis net.Listener, cfg *config.Config) error {
	logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	logger.Infof("Listening on: %s", server.http.Addr)
	switch cfg.RumConfig.IsEnabled() {
	case true:
		logger.Info("RUM endpoints enabled!")
		for _, s := range cfg.RumConfig.AllowOrigins {
			if s == "*" {
				logger.Warn("CORS related setting `apm-server.rum.allow_origins` allows all origins. Consider more restrictive setting for production use.")
				break
			}
		}
	case false:
		logger.Info("RUM endpoints disabled.")
	}

	if cfg.MaxConnections > 0 {
		lis = netutil.LimitListener(lis, cfg.MaxConnections)
		logger.Infof("Connection limit set to: %d", cfg.MaxConnections)
	}

	var serveHTTP func(net.Listener) error
	if server.http.TLSConfig != nil {
		logger.Info("SSL enabled.")
		serveHTTP = func(lis net.Listener) error {
			return server.http.ServeTLS(lis, "", "")
		}
	} else {
		if cfg.SecretToken != "" {
			logger.Warn("Secret token is set, but SSL is not enabled.")
		}
		logger.Info("SSL disabled.")
		serveHTTP = server.http.Serve
	}

	mux := cmux.New(lis)
	grpcLis := mux.MatchWithWriters(cmux.HTTP2MatchHeaderFieldPrefixSendSettings("content-type", "application/grpc"))
	httpLis := mux.Match(cmux.Any())

	g, _ := errgroup.WithContext(context.Background())
	g.Go(mux.Serve)
	g.Go(func() error { return server.grpc.Serve(grpcLis) })
	g.Go(func() error { return serveHTTP(httpLis) })
	return g.Wait()
}

func stop(logger *logp.Logger, server *http.Server) {

	err := server.Shutdown(context.Background())
	if err != nil {
		logger.Error(err.Error())
		err = server.Close()
		if err != nil {
			logger.Error(err.Error())
		}
	}
}
