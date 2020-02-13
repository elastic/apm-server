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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.elastic.co/apm"
	"go.elastic.co/apm/transport"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/api"
	"github.com/elastic/apm-server/beater/config"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/pipelistener"
	"github.com/elastic/apm-server/publish"
)

func init() {
	apm.DefaultTracer.Close()
}

// initTracer configures and returns an apm.Tracer for tracing
// the APM server's own execution. If the server is configured
// to send tracing data to itself, it will return a tracerServer
// that can be used for receiving trace data.
func initTracer(info beat.Info, cfg *config.Config, logger *logp.Logger) (*apm.Tracer, *tracerServer, error) {
	if !cfg.SelfInstrumentation.IsEnabled() {
		os.Setenv("ELASTIC_APM_ACTIVE", "false")
		logger.Infof("self instrumentation is disabled")
	} else {
		os.Setenv("ELASTIC_APM_ACTIVE", "true")
		logger.Infof("self instrumentation is enabled")
	}
	if !cfg.SelfInstrumentation.IsEnabled() {
		tracer, err := apm.NewTracer(info.Beat, info.Version)
		return tracer, nil, err
	}

	if cfg.SelfInstrumentation.Profiling.CPU.IsEnabled() {
		interval := cfg.SelfInstrumentation.Profiling.CPU.Interval
		duration := cfg.SelfInstrumentation.Profiling.CPU.Duration
		logger.Infof("CPU profiling: every %s for %s", interval, duration)
		os.Setenv("ELASTIC_APM_CPU_PROFILE_INTERVAL", fmt.Sprintf("%dms", int(interval.Seconds()*1000)))
		os.Setenv("ELASTIC_APM_CPU_PROFILE_DURATION", fmt.Sprintf("%dms", int(duration.Seconds()*1000)))
	}
	if cfg.SelfInstrumentation.Profiling.Heap.IsEnabled() {
		interval := cfg.SelfInstrumentation.Profiling.Heap.Interval
		logger.Infof("Heap profiling: every %s", interval)
		os.Setenv("ELASTIC_APM_HEAP_PROFILE_INTERVAL", fmt.Sprintf("%dms", int(interval.Seconds()*1000)))
	}

	var tracerTransport transport.Transport
	var tracerServer *tracerServer
	if cfg.SelfInstrumentation.Hosts != nil {
		// tracing destined for external host
		t, err := transport.NewHTTPTransport()
		if err != nil {
			return nil, nil, err
		}
		t.SetServerURL(cfg.SelfInstrumentation.Hosts...)
		if cfg.SelfInstrumentation.APIKey != "" {
			t.SetAPIKey(cfg.SelfInstrumentation.APIKey)
		} else {
			t.SetSecretToken(cfg.SelfInstrumentation.SecretToken)
		}
		tracerTransport = t
		logger.Infof("self instrumentation directed to %s", cfg.SelfInstrumentation.Hosts)
	} else {
		var err error
		tracerServer, err = newTracerServer(cfg)
		if err != nil {
			return nil, nil, err
		}
		tracerTransport = tracerServer.transport
	}

	var environment string
	if cfg.SelfInstrumentation.Environment != nil {
		environment = *cfg.SelfInstrumentation.Environment
	}
	tracer, err := apm.NewTracerOptions(apm.TracerOptions{
		ServiceName:        info.Beat,
		ServiceVersion:     info.Version,
		ServiceEnvironment: environment,
		Transport:          tracerTransport,
	})
	if err != nil {
		return nil, nil, err
	}
	tracer.SetLogger(logp.NewLogger(logs.Tracing))

	return tracer, tracerServer, nil
}

type tracerServer struct {
	cfg       *config.Config
	logger    *logp.Logger
	server    *http.Server
	listener  net.Listener
	transport transport.Transport
}

func newTracerServer(cfg *config.Config) (*tracerServer, error) {
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

	// Create an in-process net.Listener for the tracer. This enables us to:
	// - avoid the network stack
	// - avoid/ignore TLS for self-tracing
	// - skip tracing when the requests come from the in-process transport
	//   (i.e. to avoid recursive/repeated tracing.)
	pipeListener := pipelistener.New()
	pipeTransport, err := transport.NewHTTPTransport()
	if err != nil {
		return nil, err
	}
	pipeTransport.SetServerURL(&url.URL{Scheme: "http", Host: "localhost"})
	pipeTransport.Client.Transport = &http.Transport{
		DialContext:     pipeListener.DialContext,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}

	return &tracerServer{
		cfg:       cfg,
		logger:    logp.NewLogger(logs.Beater),
		server:    server,
		listener:  pipeListener,
		transport: pipeTransport,
	}, nil
}

func (s *tracerServer) serve(report publish.Reporter) error {
	mux, err := api.NewMux(s.cfg, report)
	if err != nil {
		return err
	}
	s.server.Handler = mux
	return s.server.Serve(s.listener)
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
