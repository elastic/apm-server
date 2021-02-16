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

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/publish"
)

type tracerServer struct {
	server   *http.Server
	logger   *logp.Logger
	requests <-chan tracerServerRequest
}

func newTracerServer(listener net.Listener, logger *logp.Logger) (*tracerServer, error) {
	requests := make(chan tracerServerRequest)
	report := func(ctx context.Context, req publish.PendingReq) error {
		result := make(chan error, 1)
		request := tracerServerRequest{ctx: ctx, req: req, res: result}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case requests <- request:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-result:
			return err
		}
	}
	cfg := config.DefaultConfig()
	mux, err := api.NewMux(beat.Info{}, cfg, report)
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		Handler:        mux,
		IdleTimeout:    cfg.IdleTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderSize,
	}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			logger.Error(err.Error())
		}
	}()
	return &tracerServer{
		server:   server,
		logger:   logger,
		requests: requests,
	}, nil
}

// Close closes the tracerServer's listener.
func (s *tracerServer) Close() error {
	return s.server.Shutdown(context.Background())
}

// serve serves event publication requests for the tracer server. This may be
// called multiple times in series, but not concurrently.
func (s *tracerServer) serve(ctx context.Context, report publish.Reporter) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case req := <-s.requests:
			req.res <- report(req.ctx, req.req)
		}
	}
}

type tracerServerRequest struct {
	ctx context.Context
	req publish.PendingReq
	res chan<- error
}
