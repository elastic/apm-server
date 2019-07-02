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

	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/libbeat/kibana"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/version"
)

func newServer(config *Config, tracer *apm.Tracer, kbClient *kibana.Client, report publish.Reporter) (*http.Server, error) {
	mux, err := newMuxer(config, kbClient, report)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Addr: config.Host,
		Handler: apmhttp.Wrap(mux,
			apmhttp.WithServerRequestIgnorer(doNotTrace),
			apmhttp.WithTracer(tracer),
		),
		IdleTimeout:    config.IdleTimeout,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		MaxHeaderBytes: config.MaxHeaderSize,
	}

	if config.TLS.IsEnabled() {
		tlsServerConfig, err := tlscommon.LoadTLSServerConfig(config.TLS)
		if err != nil {
			return nil, err
		}
		server.TLSConfig = tlsServerConfig.BuildModuleConfig(config.Host)
	}
	return server, nil
}

func doNotTrace(req *http.Request) bool {
	if req.RemoteAddr == "pipe" {
		// Don't trace requests coming from self,
		// or we will go into a continuous cycle.
		return true
	}
	if req.URL.Path == rootURL {
		// Don't trace root url (healthcheck) requests.
		return true
	}
	return false
}

func run(logger *logp.Logger, server *http.Server, lis net.Listener, config *Config) error {
	logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	logger.Infof("Listening on: %s", server.Addr)
	switch config.RumConfig.isEnabled() {
	case true:
		logger.Info("RUM endpoints enabled!")
	case false:
		logger.Info("RUM endpoints disabled.")
	}

	if config.MaxConnections > 0 {
		lis = netutil.LimitListener(lis, config.MaxConnections)
		logger.Infof("Connection limit set to: %d", config.MaxConnections)
	}

	if server.TLSConfig != nil {
		logger.Info("SSL enabled.")
		return server.ServeTLS(lis, "", "")
	}
	if config.SecretToken != "" {
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
