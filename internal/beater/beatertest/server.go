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

package beatertest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-server/internal/beater"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
)

// Server runs the core APM Server that, by default, listens on a system-chosen port
// on the loopback interface, for use in testing package beater.
type Server struct {
	group  *errgroup.Group
	ctx    context.Context
	cancel context.CancelFunc
	runner *beater.Runner

	// Logs holds the server's logs.
	Logs *observer.ObservedLogs

	// Client holds an *http.Client that is configured for sending
	// requests to the server.
	Client *http.Client

	// URL holds the base URL of the server.
	//
	// This will be set by Start and newServer.
	URL string
}

// NewServer returns a new started APM Server with the given configuration.
//
// The server's Close method will be invoked as a test cleanup.
func NewServer(t testing.TB, opts ...option) *Server {
	srv := NewUnstartedServer(t, opts...)
	err := srv.Start()
	require.NoError(t, err)
	return srv
}

// NewUnstartedServer returns a new unstarted APM Server with the given options.
//
// By default the server will send documents to no-op output. To observe documents
// as they would be sent to Elasticsearch, use ElasticsearchOutputConfig and pass the
// configuration to New(Unstarted)Server.
//
// The server's Start method should be called to start the server.
func NewUnstartedServer(t testing.TB, opts ...option) *Server {
	core, observedLogs := observer.New(zapcore.DebugLevel)
	logger := logp.NewLogger("", zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewTee(in, core)
	}))

	options := options{
		config: []*agentconfig.C{agentconfig.MustNewConfigFrom(map[string]interface{}{
			"apm-server": map[string]interface{}{
				"host": "localhost:0",

				// Disable waiting for integration to be installed by default,
				// to simplify tests. This feature is tested independently.
				"data_streams.wait_for_integration": false,
			},
		})},
	}
	for _, o := range opts {
		o(&options)
	}

	cfg, err := agentconfig.MergeConfigs(options.config...)
	require.NoError(t, err)

	// If no output is defined, use output.null.
	var outputConfig struct {
		Output agentconfig.Namespace `config:"output"`
	}
	err = cfg.Unpack(&outputConfig)
	require.NoError(t, err)
	if !outputConfig.Output.IsSet() {
		err = cfg.Merge(map[string]any{
			"output.null": map[string]any{},
		})
		require.NoError(t, err)
	}

	runner, err := beater.NewRunner(beater.RunnerParams{
		Config:     cfg,
		Logger:     logger,
		WrapServer: options.wrapServer,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	group, ctx := errgroup.WithContext(ctx)
	srv := &Server{
		group:  group,
		ctx:    ctx,
		cancel: cancel,
		runner: runner,
		Logs:   observedLogs,
	}
	t.Cleanup(func() { srv.Close() })

	return srv
}

// Start starts the server running in the background.
//
// After Start returns without error, s.URL will be set to the server's base URL.
func (s *Server) Start() error {
	exited := make(chan struct{})
	s.group.Go(func() error {
		defer close(exited)
		return s.runner.Run(s.ctx)
	})

	listenAddr, err := s.waitListenAddr(exited)
	if err != nil {
		return err
	}

	var transport http.RoundTripper
	if u, err := url.Parse(listenAddr); err == nil && u.Scheme == "unix" {
		transport = &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				return net.Dial("unix", u.Path)
			},
		}
		s.URL = "http://test-apm-server/"
	} else {
		baseURL := &url.URL{Scheme: "http", Host: listenAddr}
		s.URL = baseURL.String()
	}
	s.Client = &http.Client{Transport: transport}

	return nil

}

func (s *Server) waitListenAddr(exited <-chan struct{}) (string, error) {
	deadline := time.After(10 * time.Second)
	for {
		for _, entry := range s.Logs.All() {
			const prefix = "Listening on: "
			if strings.HasPrefix(entry.Message, prefix) {
				return entry.Message[len(prefix):], nil
			}
		}
		select {
		case <-exited:
			return "", errors.New("server exited before logging expected message")
		case <-deadline:
			return "", errors.New("timeout waiting for server to start listening")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// Close stops the server.
func (s *Server) Close() error {
	s.cancel()
	return s.group.Wait()
}

type options struct {
	config     []*agentconfig.C
	wrapServer beater.WrapServerFunc
}

type option func(*options)

// WithConfig is an option for supplying configuration to the server.
// Configuration provided by WithConfig will be merged with the base
// configuration:
//
//	apm-server:
//	  host: localhost:0
//	  data_streams.wait_for_integration: false
func WithConfig(cfg ...*agentconfig.C) option {
	return func(opts *options) {
		opts.config = append(opts.config, cfg...)
	}
}

// WithWrapServer is an option for setting a WrapServerFunc.
func WithWrapServer(wrapServer beater.WrapServerFunc) option {
	return func(opts *options) {
		opts.wrapServer = wrapServer
	}
}
