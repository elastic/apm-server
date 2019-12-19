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
	"fmt"
	"net"
	"net/http"
	"time"

	"go.elastic.co/apm"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/version"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/jaeger"
	"github.com/elastic/apm-server/publish"
)

type server struct {
	logger *logp.Logger
	cfg    *config.Config

	httpServer       *httpServer
	jaegerGRPCServer *jaeger.GRPCServer
}

func newServer(logger *logp.Logger, cfg *config.Config, tracer *apm.Tracer, reporter publish.Reporter) (server, error) {
	httpServer, err := newHTTPServer(logger, cfg, tracer, reporter)
	if err != nil {
		return server{}, err
	}
	jaegerGRPCServer, err := jaeger.NewGRPCServer(logger, cfg, tracer, reporter)
	if err != nil {
		return server{}, err
	}
	return server{logger, cfg, httpServer, jaegerGRPCServer}, nil
}

func (s server) run(listener, traceListener net.Listener, publish func(beat.Event)) error {
	s.logger.Infof("Starting apm-server [%s built %s]. Hit CTRL-C to stop it.", version.Commit(), version.BuildTime())
	var g errgroup.Group

	if s.jaegerGRPCServer != nil {
		g.Go(func() error {
			return s.jaegerGRPCServer.Start()
		})
	}
	if s.httpServer != nil {
		g.Go(func() error {
			return s.httpServer.start(listener)
		})

		if s.isAvailable() {
			go notifyListening(s.cfg, publish)
		}

		if traceListener != nil {
			g.Go(func() error {
				return s.httpServer.Serve(traceListener)
			})
		}

		if err := g.Wait(); err != http.ErrServerClosed {
			return err
		}

		s.logger.Infof("Server stopped")
	}

	return nil
}

func (s server) stop(logger *logp.Logger) {
	if s.jaegerGRPCServer != nil {
		s.jaegerGRPCServer.Stop()
	}
	if s.httpServer != nil {
		s.httpServer.stop()
	}
}

func (s server) isAvailable() bool {
	// following an example from https://golang.org/pkg/net/
	// dial into tcp connection to ensure listener is ready, send get request and read response,
	// in case tls is enabled, the server will respond with 400,
	// as this only checks the server is up and reachable errors can be ignored
	timeout := s.cfg.ShutdownTimeout
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
