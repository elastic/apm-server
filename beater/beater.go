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
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/transport"
	"github.com/elastic/apm-server/ingest/pipeline"
	"github.com/elastic/apm-server/pipelistener"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs/elasticsearch"
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
	logger := logp.NewLogger("beater")
	beaterConfig, err := NewConfig(b.Info.Version, ucfg)
	if err != nil {
		return nil, err
	}
	if beaterConfig.RumConfig.isEnabled() {
		if b.Config != nil && beaterConfig.RumConfig.SourceMapping.EsConfig == nil {
			// fall back to elasticsearch output configuration for sourcemap storage if possible
			if isElasticsearchOutput(b) {
				logger.Info("Falling back to elasticsearch output for sourcemap storage")
				beaterConfig.setSmapElasticsearch(b.Config.Output.Config())
			} else {
				logger.Info("Unable to determine sourcemap storage, sourcemaps will not be applied")
			}
		}
	}

	bt := &beater{
		config:  beaterConfig,
		stopped: false,
		logger:  logger,
	}

	if isElasticsearchOutput(b) && beaterConfig.Register.Ingest.Pipeline.isEnabled() {
		logger.Info("Registering pipeline callback.")
		err := bt.registerPipelineCallback(b)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("No pipeline callback registered")
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
	if traceListener != nil {
		defer traceListener.Close()
	}
	defer tracer.Close()

	pub, err := publish.NewPublisher(b.Publisher, bt.config.ConcurrentRequests, bt.config.ShutdownTimeout, tracer)
	if err != nil {
		return err
	}
	defer pub.Stop()

	lis, err := bt.listen()
	if err != nil {
		bt.logger.Error("failed to listen:", err)
		return nil
	}

	go notifyListening(bt.config, pub.Client().Publish)

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
	if traceListener != nil {
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
	// tracing disabled, setup complete
	if !config.SelfInstrumentation.isEnabled() {
		return tracer, nil, nil
	}

	if config.SelfInstrumentation.Environment != nil {
		tracer.Service.Environment = *config.SelfInstrumentation.Environment
	}
	tracer.SetLogger(logp.NewLogger("tracing"))

	// tracing destined for external host
	// only first host used until https://github.com/elastic/apm-agent-go/issues/200
	if config.SelfInstrumentation.Hosts != nil {
		t, err := transport.NewHTTPTransport(config.SelfInstrumentation.Hosts[0], config.SelfInstrumentation.SecretToken)
		if err != nil {
			tracer.Close()
			return nil, nil, err
		}
		tracer.Transport = t
		logger.Infof("self instrumentation directed to %s", config.SelfInstrumentation.Hosts[0])

		return tracer, nil, nil
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

func isElasticsearchOutput(b *beat.Beat) bool {
	return b.Config != nil && b.Config.Output.Name() == "elasticsearch"
}

func (bt *beater) registerPipelineCallback(b *beat.Beat) error {
	overwrite := bt.config.Register.Ingest.Pipeline.shouldOverwrite()
	path := bt.config.Register.Ingest.Pipeline.Path

	// ensure setup cmd is working properly
	b.OverwritePipelinesCallback = func(esConfig *common.Config) error {
		esClient, err := elasticsearch.NewConnectedClient(esConfig)
		if err != nil {
			return err
		}
		return pipeline.RegisterPipelines(esClient, overwrite, path)
	}
	// ensure pipelines are registered when new ES connection is established.
	elasticsearch.RegisterConnectCallback(func(esClient *elasticsearch.Client) error {
		return pipeline.RegisterPipelines(esClient, overwrite, path)
	})
	return nil
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
