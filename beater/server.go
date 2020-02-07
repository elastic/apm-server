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
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model/onboarding"

	"go.elastic.co/apm"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/version"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/jaeger"
	"github.com/elastic/apm-server/publish"
)

type server struct {
	logger *logp.Logger
	cfg    *config.Config

	httpServer   *httpServer
	jaegerServer *jaeger.Server
	reporter     publish.Reporter
}

func newServer(logger *logp.Logger, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter) (server, error) {
	httpServer, err := newHTTPServer(logger, cfg, tracer, reporter)
	if err != nil {
		return server{}, err
	}
	jaegerServer, err := jaeger.NewServer(logger, cfg, tracer, reporter)
	if err != nil {
		return server{}, err
	}
	return server{
		logger:       logger,
		cfg:          cfg,
		httpServer:   httpServer,
		jaegerServer: jaegerServer,
		reporter:     reporter,
	}, nil
}

func (s server) run(listener net.Listener, tracerServer *tracerServer) error {
	s.logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	var g errgroup.Group
	if s.jaegerServer != nil {
		g.Go(s.jaegerServer.Serve)
	}
	if s.httpServer != nil {
		g.Go(func() error {
			return s.httpServer.start(listener)
		})
		if s.isAvailable(s.cfg.ShutdownTimeout) {
			notifyListening(context.Background(), s.cfg, s.reporter)
		}
		if tracerServer != nil {
			g.Go(func() error {
				return tracerServer.serve(s.reporter)
			})
		}
	}
	if err := g.Wait(); err != http.ErrServerClosed {
		return err
	}
	s.logger.Infof("Server stopped")
	return nil
}

func (s server) stop() {
	if s.jaegerServer != nil {
		s.jaegerServer.Stop()
	}
	if s.httpServer != nil {
		s.httpServer.stop()
	}
}

func notifyListening(ctx context.Context, config *config.Config, reporter publish.Reporter) {
	logp.NewLogger(logs.Onboarding).Info("Publishing onboarding document")
	reporter(ctx, publish.PendingReq{
		Transformables: []publish.Transformable{onboarding.Doc{ListenAddr: config.Host}},
	})
}

func (s server) isAvailable(timeout time.Duration) bool {
	// following an example from https://golang.org/pkg/net/
	// dial into tcp connection to ensure listener is ready, send get request and read response,
	// in case tls is enabled, the server will respond with 400,
	// as this only checks the server is up and reachable errors can be ignored
	conn, err := net.DialTimeout("tcp", s.cfg.Host, timeout)
	if err != nil {
		return false
	}
	err = conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return false
	}
	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	_, err = bufio.NewReader(conn).ReadByte()
	if err != nil {
		return false
	}

	err = conn.Close()
	return err == nil
}
