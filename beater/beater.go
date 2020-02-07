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
	"errors"
	"net"
	"net/url"
	"sync"

	"go.elastic.co/apm"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/cfgfile"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs/elasticsearch"

	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/ingest/pipeline"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/publish"
)

func init() {
	apm.DefaultTracer.Close()
}

type beater struct {
	config   *config.Config
	logger   *logp.Logger
	mutex    sync.Mutex // guards server and stopped
	server   server
	stopping chan struct{}
	stopped  bool
}

var (
	errSetupDashboardRemoved = errors.New("setting 'setup.dashboards' has been removed")
)

// checkConfig verifies the global configuration doesn't use unsupported settings
func checkConfig(logger *logp.Logger) error {
	cfg, err := cfgfile.Load("", nil)
	if err != nil {
		// responsibility for failing to load configuration lies elsewhere
		// this is not reachable after going through normal beat creation
		return nil
	}

	var s struct {
		Dashboards *common.Config `config:"setup.dashboards"`
	}
	if err := cfg.Unpack(&s); err != nil {
		return err
	}
	if s.Dashboards != nil {
		if s.Dashboards.Enabled() {
			return errSetupDashboardRemoved
		}
		logger.Warn(errSetupDashboardRemoved)
	}
	return nil
}

// New creates a beater instance using the provided configuration
func New(b *beat.Beat, ucfg *common.Config) (beat.Beater, error) {
	logger := logp.NewLogger(logs.Beater)
	if err := checkConfig(logger); err != nil {
		return nil, err
	}
	var esOutputCfg *common.Config
	if isElasticsearchOutput(b) {
		esOutputCfg = b.Config.Output.Config()
	}

	beaterConfig, err := config.NewConfig(b.Info.Version, ucfg, esOutputCfg)
	if err != nil {
		return nil, err
	}

	bt := &beater{
		config:   beaterConfig,
		stopping: make(chan struct{}),
		stopped:  false,
		logger:   logger,
	}

	// setup pipelines if explicitly directed to or setup --pipelines and config is not set at all
	shouldSetupPipelines := beaterConfig.Register.Ingest.Pipeline.IsEnabled() ||
		(b.InSetupCmd && beaterConfig.Register.Ingest.Pipeline.Enabled == nil)
	if isElasticsearchOutput(b) && shouldSetupPipelines {
		logger.Info("Registering pipeline callback")
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
			path = net.JoinHostPort(path, config.DefaultPort)
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
	tracer, tracerServer, err := initTracer(b.Info, bt.config, bt.logger)
	if err != nil {
		return err
	}
	if tracerServer != nil {
		go func() {
			defer tracerServer.stop()
			<-bt.stopping
		}()
	}
	defer tracer.Close()
	pub, err := publish.NewPublisher(b.Publisher, tracer, bt.config, b.Info)
	if err != nil {
		return err
	}
	defer pub.Stop()

	lis, err := bt.listen()
	if err != nil {
		bt.logger.Error("failed to listen:", err)
		return nil
	}

	bt.mutex.Lock()
	if bt.stopped {
		bt.mutex.Unlock()
		return nil
	}

	bt.server, err = newServer(bt.logger, bt.config, tracer, pub.Send)
	if err != nil {
		bt.logger.Error("failed to create new server:", err)
		return nil
	}
	bt.mutex.Unlock()

	//blocking until shutdown
	return bt.server.run(lis, tracerServer)
}

func isElasticsearchOutput(b *beat.Beat) bool {
	return b.Config != nil && b.Config.Output.Name() == "elasticsearch"
}

func (bt *beater) registerPipelineCallback(b *beat.Beat) error {
	overwrite := bt.config.Register.Ingest.Pipeline.ShouldOverwrite()
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
	_, err := elasticsearch.RegisterConnectCallback(func(esClient *elasticsearch.Client) error {
		return pipeline.RegisterPipelines(esClient, overwrite, path)
	})
	return err
}

// Graceful shutdown
func (bt *beater) Stop() {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()
	if bt.stopped {
		return
	}
	bt.logger.Infof("stopping apm-server... waiting maximum of %v seconds for queues to drain",
		bt.config.ShutdownTimeout.Seconds())
	bt.server.stop()
	close(bt.stopping)
	bt.stopped = true
}
