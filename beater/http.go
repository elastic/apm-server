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
	"net/url"

	"go.elastic.co/apm"
	"go.elastic.co/apm/module/apmhttp"
	"golang.org/x/net/netutil"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/gmux"
)

type httpServer struct {
	*http.Server
	cfg          *config.Config
	logger       *logp.Logger
	reporter     publish.Reporter
	grpcListener net.Listener
}

func newHTTPServer(logger *logp.Logger, info beat.Info, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter) (*httpServer, error) {
	mux, err := api.NewMux(info, cfg, reporter)
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
		server.TLSConfig = tlsServerConfig.BuildServerConfig("")
	}

	// Configure the server with gmux. The returned net.Listener will receive
	// gRPC connections, while all other requests will be handled by s.Handler.
	//
	// grpcListener is closed when the HTTP server is shutdown.
	grpcListener, err := gmux.ConfigureServer(server, nil)
	if err != nil {
		return nil, err
	}

	return &httpServer{server, cfg, logger, reporter, grpcListener}, nil
}

func (h *httpServer) start() error {
	lis, err := h.listen()
	if err != nil {
		return err
	}
	addr := lis.Addr()
	if addr.Network() == "tcp" {
		h.logger.Infof("Listening on: %s", addr)
	} else {
		h.logger.Infof("Listening on: %s:%s", addr.Network(), addr.String())
	}

	switch h.cfg.RumConfig.IsEnabled() {
	case true:
		h.logger.Info("RUM endpoints enabled!")
		for _, s := range h.cfg.RumConfig.AllowOrigins {
			if s == "*" {
				h.logger.Warn("CORS related setting `apm-server.rum.allow_origins` allows all origins. Consider more restrictive setting for production use.")
				break
			}
		}
	case false:
		h.logger.Info("RUM endpoints disabled.")
	}

	if h.cfg.MaxConnections > 0 {
		lis = netutil.LimitListener(lis, h.cfg.MaxConnections)
		h.logger.Infof("Connection limit set to: %d", h.cfg.MaxConnections)
	}

	if !h.cfg.DataStreams.Enabled {
		// Create the "onboarding" document, which contains the server's
		// listening address. We only do this if data streams are not enabled,
		// as onboarding documents are incompatible with data streams.
		// Onboarding documents should be replaced by Fleet status later.
		notifyListening(context.Background(), addr, h.reporter)
	}

	if h.cfg.TLS.IsEnabled() {
		h.logger.Info("SSL enabled.")
		return h.ServeTLS(lis, "", "")
	}
	if h.cfg.SecretToken != "" {
		h.logger.Warn("Secret token is set, but SSL is not enabled.")
	}
	h.logger.Info("SSL disabled.")

	return h.Serve(lis)
}

func (h *httpServer) stop() {
	h.logger.Infof("Stop listening on: %s", h.Server.Addr)
	if err := h.Shutdown(context.Background()); err != nil {
		h.logger.Errorf("error stopping http server: %s", err.Error())
		if err := h.Close(); err != nil {
			h.logger.Errorf("error closing http server: %s", err.Error())
		}
	}
}

// listen starts the listener for bt.config.Host.
func (h *httpServer) listen() (net.Listener, error) {
	if url, err := url.Parse(h.cfg.Host); err == nil && url.Scheme == "unix" {
		return net.Listen("unix", url.Path)
	}

	const network = "tcp"
	addr := h.cfg.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		// Tack on a port if SplitHostPort fails on what should be a
		// tcp network address. If splitting failed because there were
		// already too many colons, one more won't change that.
		addr = net.JoinHostPort(addr, config.DefaultPort)
	}
	return net.Listen(network, addr)
}

func doNotTrace(req *http.Request) bool {
	// Don't trace root url (healthcheck) requests.
	return req.URL.Path == api.RootPath
}
