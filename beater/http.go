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
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/libp2p/go-reuseport"
	"go.uber.org/zap"
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
	httpListener net.Listener
}

func newHTTPServer(
	logger *logp.Logger,
	info beat.Info,
	cfg *config.Config,
	handler http.Handler,
	reporter publish.Reporter,
	listener net.Listener,
) (*httpServer, error) {

	server := &http.Server{
		Addr:           cfg.Host,
		Handler:        handler,
		IdleTimeout:    cfg.IdleTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderSize,
		ErrorLog:       newErrorLog(logger),
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

	return &httpServer{server, cfg, logger, reporter, grpcListener, listener}, nil
}

func (h *httpServer) start() error {
	if h.cfg.RumConfig.Enabled {
		h.logger.Info("RUM endpoints enabled!")
		for _, s := range h.cfg.RumConfig.AllowOrigins {
			if s == "*" {
				h.logger.Warn("CORS related setting `apm-server.rum.allow_origins` allows all origins. Consider more restrictive setting for production use.")
				break
			}
		}
	} else {
		h.logger.Info("RUM endpoints disabled.")
	}

	if !h.cfg.DataStreams.Enabled {
		// Create the "onboarding" document, which contains the server's
		// listening address. We only do this if data streams are not enabled,
		// as onboarding documents are incompatible with data streams.
		// Onboarding documents should be replaced by Fleet status later.
		notifyListening(context.Background(), h.httpListener.Addr(), h.reporter)
	}

	if h.cfg.TLS.IsEnabled() {
		h.logger.Info("SSL enabled.")
		return h.ServeTLS(h.httpListener, "", "")
	}
	if h.cfg.AgentAuth.SecretToken != "" {
		h.logger.Warn("Secret token is set, but SSL is not enabled.")
	}
	h.logger.Info("SSL disabled.")

	return h.Serve(h.httpListener)
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
func listen(cfg *config.Config, logger *logp.Logger) (net.Listener, error) {
	var listener net.Listener
	url, err := url.Parse(cfg.Host)
	if err == nil && url.Scheme == "unix" {
		// SO_REUSEPORT does not support unix sockets
		listener, err = net.Listen("unix", url.Path)
	} else {
		addr := cfg.Host
		if _, _, err := net.SplitHostPort(addr); err != nil {
			// Tack on a port if SplitHostPort fails on what should be a
			// tcp network address. If splitting failed because there were
			// already too many colons, one more won't change that.
			addr = net.JoinHostPort(addr, config.DefaultPort)
		}
		listener, err = reuseport.Listen("tcp", addr)
	}
	if err != nil {
		return nil, err
	}

	addr := listener.Addr()
	if network := addr.Network(); network == "tcp" {
		logger.Infof("Listening on: %s", addr)
	} else {
		logger.Infof("Listening on: %s:%s", network, addr.String())
	}
	if cfg.MaxConnections > 0 {
		logger.Infof("Connection limit set to: %d", cfg.MaxConnections)
		listener = netutil.LimitListener(listener, cfg.MaxConnections)
	}
	return listener, nil
}

func doNotTrace(req *http.Request) bool {
	// Don't trace root url (healthcheck) requests.
	return req.URL.Path == api.RootPath
}

// newErrorLog returns a standard library log.Logger that sends
// logs to logger with error level.
func newErrorLog(logger *logp.Logger) *log.Logger {
	logger = logger.Named("http")
	logger = logger.WithOptions(zap.AddCallerSkip(3))
	w := errorLogWriter{logger}
	return log.New(w, "", 0)
}

type errorLogWriter struct {
	logger *logp.Logger
}

func (w errorLogWriter) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	w.logger.Error(message)
	return len(p), nil
}
