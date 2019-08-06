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

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/net/netutil"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/version"
)

func newServer(cfg *config.Config, tracer *apm.Tracer, report publish.Reporter) (*http.Server, error) {
	mux, err := api.NewMux(cfg, report)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
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
		server.TLSConfig = tlsServerConfig.BuildModuleConfig(cfg.Host)
	}
	return server, nil
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

func run(logger *logp.Logger, server *http.Server, lis net.Listener, cfg *config.Config) error {
	logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	logger.Infof("Listening on: %s", server.Addr)
	switch cfg.RumConfig.IsEnabled() {
	case true:
		logger.Info("RUM endpoints enabled!")
		for _, s := range config.RumConfig.AllowOrigins {
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

	if server.TLSConfig != nil {
		logger.Info("SSL enabled.")
		return server.ServeTLS(lis, "", "")
	}
	if cfg.SecretToken != "" {
		logger.Warn("Secret token is set, but SSL is not enabled.")
	} else {
		logger.Info("SSL disabled.")
	}
	return server.Serve(lis)
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
