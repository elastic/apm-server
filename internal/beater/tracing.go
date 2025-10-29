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
	"net"
	"net/http"

	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"

	"github.com/elastic/apm-data/input"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/internal/agentcfg"
	"github.com/elastic/apm-server/internal/beater/api"
	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/config"
	"github.com/elastic/apm-server/internal/beater/ratelimit"
)

func newTracerServer(cfg *config.Config, listener net.Listener, logger *logp.Logger, batchProcessor modelpb.BatchProcessor, semaphore input.Semaphore, mp metric.MeterProvider) (*http.Server, error) {
	ratelimitStore, err := ratelimit.NewStore(1, 1, 1) // unused, arbitrary params
	if err != nil {
		return nil, err
	}
	authenticator, err := auth.NewAuthenticator(config.AgentAuth{}, nooptrace.NewTracerProvider(), logger)
	if err != nil {
		return nil, err
	}
	mux, err := api.NewMux(
		cfg,
		batchProcessor,
		authenticator,
		agentcfg.NewEmptyFetcher(),
		ratelimitStore,
		nil,                         // no sourcemap store
		func() bool { return true }, // ready for publishing
		semaphore,
		noopmetric.NewMeterProvider(),
		nooptrace.NewTracerProvider(),
		logger,
		monitoring.NewRegistry(), // unused
	)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Handler:        mux,
		IdleTimeout:    cfg.IdleTimeout,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderSize,
	}, nil
}
