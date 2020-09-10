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

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/publish"
)

func init() {
	apm.DefaultTracer.Close()
}

type tracerServer struct {
	cfg      *config.Config
	logger   *logp.Logger
	server   *http.Server
	listener net.Listener
}

func newTracerServer(cfg *config.Config, listener net.Listener) *tracerServer {
	if listener == nil {
		return nil
	}

	cfgCopy := *cfg // Copy cfg so we can disable auth
	cfg = &cfgCopy
	cfg.SecretToken = ""
	cfg.APIKeyConfig = nil

	server := &http.Server{
		IdleTimeout:    cfg.IdleTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderSize,
	}

	return &tracerServer{
		cfg:      cfg,
		logger:   logp.NewLogger(logs.Beater),
		server:   server,
		listener: listener,
	}
}

func (s *tracerServer) serve(report publish.Reporter) error {
	mux, err := api.NewMux(s.cfg, report)
	if err != nil {
		return err
	}
	s.server.Handler = mux
	if err := s.server.Serve(s.listener); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *tracerServer) stop() {
	err := s.server.Shutdown(context.Background())
	if err != nil {
		s.logger.Error(err.Error())
		if err := s.server.Close(); err != nil {
			s.logger.Error(err.Error())
		}
	}
}
