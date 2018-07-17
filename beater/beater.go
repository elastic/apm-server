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
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/transport"
	"github.com/elastic/apm-server/pipelistener"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

type beater struct {
	config  *Config
	mutex   sync.Mutex // guards server and stopped
	server  *http.Server
	stopped bool
	logger  *logp.Logger
}

// Creates beater
func New(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	beaterConfig := defaultConfig(b.Info.Version)
	if err := ucfg.Unpack(beaterConfig); err != nil {
		return nil, errors.Wrap(err, "Error processing configuration")
	}
	if beaterConfig.Frontend.isEnabled() {
		if _, err := regexp.Compile(beaterConfig.Frontend.LibraryPattern); err != nil {
			return nil, errors.New(fmt.Sprintf("Invalid regex for `library_pattern`: %v", err.Error()))
		}
		if _, err := regexp.Compile(beaterConfig.Frontend.ExcludeFromGrouping); err != nil {
			return nil, errors.New(fmt.Sprintf("Invalid regex for `exclude_from_grouping`: %v", err.Error()))
		}
		if b.Config != nil && b.Config.Output.Name() == "elasticsearch" {
			beaterConfig.setElasticsearch(b.Config.Output.Config())
		}
	}

	bt := &beater{
		config:  beaterConfig,
		stopped: false,
		logger:  logp.NewLogger("beater"),
	}
	return bt, nil
}

// parseListener extracts the network and path for a configured host address
// all paths are tcp unix:/path/to.sock
func parseListener(host string) (string, string) {
	if parsed, err := url.Parse(host); err == nil && parsed.Scheme == "unix" {
		return parsed.Scheme, parsed.Path
	}
	return "tcp", host
}

// listen starts the listener for bt.config.Host
// bt.config.Host may be mutated by this function in case the resolved listening address does not match the
// configured bt.config.Host value.
// This should only be called once, from Run.
func (bt *beater) listen() (net.Listener, error) {
	network, path := parseListener(bt.config.Host)
	if network == "tcp" {
		if _, _, err := net.SplitHostPort(path); err != nil {
			// tack on a port if SplitHostPort fails on what should be a tcp network address
			// if there were already too many colons, one more won't hurt
			path = net.JoinHostPort(path, defaultPort)
		}
	}
	lis, err := net.Listen(network, path)
	if err != nil {
		return nil, err
	}
	// in case host is :0 or similar
	if network == "tcp" {
		addr := lis.Addr().(*net.TCPAddr).String()
		if bt.config.Host != addr {
			bt.logger.Infof("host resolved from %s to %s", bt.config.Host, addr)
			bt.config.Host = addr
		}
	}
	return lis, err
}

func (bt *beater) Run(b *beat.Beat) error {
	tracer, traceListener, err := initTracer(b.Info, bt.config, bt.logger)
	if err != nil {
		return err
	}
	defer traceListener.Close()
	defer tracer.Close()

	pub, err := newPublisher(b.Publisher, bt.config.ConcurrentRequests, bt.config.ShutdownTimeout, tracer)
	if err != nil {
		return err
	}
	defer pub.Stop()

	lis, err := bt.listen()
	if err != nil {
		bt.logger.Error("failed to listen:", err)
		return nil
	}

	go notifyListening(bt.config, pub.client.Publish)

	bt.mutex.Lock()
	if bt.stopped {
		defer bt.mutex.Unlock()
		return nil
	}

	bt.server = newServer(bt.config, tracer, pub.Send)
	bt.mutex.Unlock()

	var g errgroup.Group
	g.Go(func() error {
		return run(bt.server, lis, bt.config)
	})
	if bt.config.SelfInstrumentation.isEnabled() {
		g.Go(func() error {
			return bt.server.Serve(traceListener)
		})
	}
	if err := g.Wait(); err != http.ErrServerClosed {
		return err
	}
	bt.logger.Infof("Server stopped")
	return nil
}

// initTracer configures and returns an elasticapm.Tracer for tracing
// the APM server's own execution.
func initTracer(info beat.Info, config *Config, logger *logp.Logger) (*elasticapm.Tracer, net.Listener, error) {
	if !config.SelfInstrumentation.isEnabled() {
		os.Setenv("ELASTIC_APM_ACTIVE", "false")
		logger.Infof("self instrumentation is disabled")
	} else {
		os.Setenv("ELASTIC_APM_ACTIVE", "true")
		logger.Infof("self instrumentation is enabled")
	}

	tracer, err := elasticapm.NewTracer(info.Beat, info.Version)
	if err != nil {
		return nil, nil, err
	}
	if config.SelfInstrumentation.isEnabled() {
		if config.SelfInstrumentation.Environment != nil {
			tracer.Service.Environment = *config.SelfInstrumentation.Environment
		}
		tracer.SetLogger(logp.NewLogger("tracing"))
	}

	// Create an in-process net.Listener for the tracer. This enables us to:
	// - avoid the network stack
	// - avoid/ignore TLS for self-tracing
	// - skip tracing when the requests come from the in-process transport
	//   (i.e. to avoid recursive/repeated tracing.)
	lis := pipelistener.New()
	transport, err := transport.NewHTTPTransport("http://localhost", config.SecretToken)
	if err != nil {
		tracer.Close()
		lis.Close()
		return nil, nil, err
	}
	transport.Client.Transport = &http.Transport{
		DialContext:     lis.DialContext,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}
	tracer.Transport = transport
	return tracer, lis, nil
}

// Graceful shutdown
func (bt *beater) Stop() {
	bt.logger.Infof("stopping apm-server... waiting maximum of %v seconds for queues to drain",
		bt.config.ShutdownTimeout.Seconds())
	bt.mutex.Lock()
	if bt.server != nil {
		stop(bt.server)
	}
	bt.stopped = true
	bt.mutex.Unlock()
}
