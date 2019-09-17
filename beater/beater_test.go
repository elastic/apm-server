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
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/transport/tlscommon"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/outputs"
	pubs "github.com/elastic/beats/libbeat/publisher"
	"github.com/elastic/beats/libbeat/publisher/pipeline"
	"github.com/elastic/beats/libbeat/publisher/processing"
	"github.com/elastic/beats/libbeat/publisher/queue"
	"github.com/elastic/beats/libbeat/publisher/queue/memqueue"

	"github.com/elastic/apm-server/beater/config"
)

func TestBeatConfig(t *testing.T) {
	falsy, truthy := false, true

	tests := map[string]struct {
		conf       map[string]interface{}
		beaterConf *config.Config
	}{
		"default config": {
			conf:       map[string]interface{}{},
			beaterConf: config.DefaultConfig("6.2.0"),
		},
		"overwrite default config": {
			conf: map[string]interface{}{
				"host":                  "localhost:3000",
				"max_header_size":       8,
				"max_event_size":        100,
				"idle_timeout":          5 * time.Second,
				"read_timeout":          3 * time.Second,
				"write_timeout":         4 * time.Second,
				"shutdown_timeout":      9 * time.Second,
				"capture_personal_data": true,
				"secret_token":          "1234random",
				"ssl": map[string]interface{}{
					"enabled":               true,
					"key":                   "1234key",
					"certificate":           "1234cert",
					"client_authentication": "none",
				},
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"rum": map[string]interface{}{
					"enabled": true,
					"event_rate": map[string]interface{}{
						"limit":    7200,
						"lru_size": 2000,
					},
					"allow_origins": []string{"example*"},
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 8 * time.Minute,
						},
						"index_pattern": "apm-test*",
					},
					"library_pattern":       "^custom",
					"exclude_from_grouping": "^grouping",
				},
				"register": map[string]interface{}{
					"ingest": map[string]interface{}{
						"pipeline": map[string]interface{}{
							"overwrite": false,
							"path":      filepath.Join("tmp", "definition.json"),
						},
					},
				},
				"kibana":                        map[string]interface{}{"enabled": "true"},
				"agent.config.cache.expiration": "2m",
			},
			beaterConf: &config.Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   8,
				MaxEventSize:    100,
				IdleTimeout:     5000000000,
				ReadTimeout:     3000000000,
				WriteTimeout:    4000000000,
				ShutdownTimeout: 9000000000,
				SecretToken:     "1234random",
				TLS: &tlscommon.ServerConfig{
					Enabled:     &truthy,
					Certificate: outputs.CertificateConfig{Certificate: "1234cert", Key: "1234key"},
					ClientAuth:  0},
				AugmentEnabled: true,
				Expvar: &config.ExpvarConfig{
					Enabled: &truthy,
					URL:     "/debug/vars",
				},
				RumConfig: &config.RumConfig{
					Enabled: &truthy,
					EventRate: &config.EventRate{
						Limit:   7200,
						LruSize: 2000,
					},
					AllowOrigins: []string{"example*"},
					SourceMapping: &config.SourceMapping{
						Cache:        &config.Cache{Expiration: 8 * time.Minute},
						IndexPattern: "apm-test*",
					},
					LibraryPattern:      "^custom",
					ExcludeFromGrouping: "^grouping",
					BeatVersion:         "6.2.0",
				},
				Register: &config.RegisterConfig{
					Ingest: &config.IngestConfig{
						Pipeline: &config.PipelineConfig{
							Enabled:   &truthy,
							Overwrite: &falsy,
							Path:      filepath.Join("tmp", "definition.json"),
						},
					},
				},
				Kibana:      common.MustNewConfigFrom(map[string]interface{}{"enabled": "true"}),
				AgentConfig: &config.AgentConfig{Cache: &config.Cache{Expiration: 2 * time.Minute}},
				Pipeline:    config.DefaultAPMPipeline,
			},
		},
		"merge config with default": {
			conf: map[string]interface{}{
				"host":         "localhost:3000",
				"secret_token": "1234random",
				"ssl": map[string]interface{}{
					"enabled": true,
				},
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"rum": map[string]interface{}{
					"enabled": true,
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 7,
						},
					},
					"library_pattern": "rum",
				},
				"register": map[string]interface{}{
					"ingest": map[string]interface{}{
						"pipeline": map[string]interface{}{
							"enabled": false,
						},
					},
				},
			},
			beaterConf: &config.Config{
				Host:            "localhost:3000",
				MaxHeaderSize:   1048576,
				MaxEventSize:    307200,
				IdleTimeout:     45000000000,
				ReadTimeout:     30000000000,
				WriteTimeout:    30000000000,
				ShutdownTimeout: 5000000000,
				SecretToken:     "1234random",
				TLS: &tlscommon.ServerConfig{
					Enabled:     &truthy,
					Certificate: outputs.CertificateConfig{Certificate: "", Key: ""},
					ClientAuth:  3},
				AugmentEnabled: true,
				Expvar: &config.ExpvarConfig{
					Enabled: &truthy,
					URL:     "/debug/vars",
				},
				RumConfig: &config.RumConfig{
					Enabled: &truthy,
					EventRate: &config.EventRate{
						Limit:   300,
						LruSize: 1000,
					},
					AllowOrigins: []string{"*"},
					SourceMapping: &config.SourceMapping{
						Cache: &config.Cache{
							Expiration: 7 * time.Second,
						},
						IndexPattern: "apm-*-sourcemap*",
					},
					LibraryPattern:      "rum",
					ExcludeFromGrouping: "^/webpack",
					BeatVersion:         "6.2.0",
				},
				Register: &config.RegisterConfig{
					Ingest: &config.IngestConfig{
						Pipeline: &config.PipelineConfig{
							Enabled: &falsy,
							Path:    filepath.Join("ingest", "pipeline", "definition.json"),
						},
					},
				},
				Kibana:      common.MustNewConfigFrom(map[string]interface{}{"enabled": "false"}),
				AgentConfig: &config.AgentConfig{Cache: &config.Cache{Expiration: 30 * time.Second}},
				Pipeline:    config.DefaultAPMPipeline,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ucfgConfig, err := common.NewConfigFrom(test.conf)
			assert.NoError(t, err)
			btr, err := New(&beat.Beat{Info: beat.Info{Version: "6.2.0"}}, ucfgConfig)
			assert.NoError(t, err)
			assert.NotNil(t, btr)
			bt := btr.(*beater)
			assert.Equal(t, test.beaterConf, bt.config)
		})
	}
}

/*
Run the benchmarks as follows:

	$ go test beater/*.go -run=XXX -bench=. -cpuprofile=cpu.out

then load the cpu profile file:

	$ go tool pprof beater.test cpu.out

type `web` to get a nice svg that shows the call graph and time spent:

	(pprof) web

To get a memory profile, use this:

	$ go test beater/*.go -run=XXX -bench=. -memprofile=mem.out

*/

type ChanClient struct {
	done    chan struct{}
	Channel chan beat.Event
}

func NewChanClientWith(ch chan beat.Event) *ChanClient {
	if ch == nil {
		ch = make(chan beat.Event, 1)
	}
	c := &ChanClient{
		done:    make(chan struct{}),
		Channel: ch,
	}
	return c
}

func (c *ChanClient) Close() error {
	close(c.done)
	return nil
}

// Publish will publish every event in the batch on the channel. Options will be ignored.
// Always returns without error.
func (c *ChanClient) Publish(batch pubs.Batch) error {
	for _, event := range batch.Events() {
		select {
		case <-c.done:
		case c.Channel <- event.Content:
		}
	}
	batch.ACK()
	return nil
}

func (c *ChanClient) String() string {
	return "event capturing test client"
}

type DummyOutputClient struct {
}

func (d *DummyOutputClient) Publish(batch pubs.Batch) error {
	batch.ACK()
	return nil
}
func (d *DummyOutputClient) Close() error   { return nil }
func (d *DummyOutputClient) String() string { return "" }

func DummyPipeline(cfg *common.Config, info beat.Info, clients ...outputs.Client) *pipeline.Pipeline {
	if len(clients) == 0 {
		clients = []outputs.Client{&DummyOutputClient{}}
	}
	if cfg == nil {
		cfg = common.NewConfig()
	}
	processors, err := processing.MakeDefaultObserverSupport(false)(info, logp.NewLogger("testbeat"), cfg)
	if err != nil {
		panic(err)
	}
	p, err := pipeline.New(
		info,
		pipeline.Monitors{},
		func(e queue.Eventer) (queue.Queue, error) {
			return memqueue.NewBroker(nil, memqueue.Settings{
				Eventer: e,
				Events:  20,
			}), nil
		},
		outputs.Group{
			Clients:   clients,
			BatchSize: 5,
			Retry:     0, // no retry. on error drop events
		},
		pipeline.Settings{
			WaitClose:     0,
			WaitCloseMode: pipeline.NoWaitOnClose,
			Processors:    processors,
		},
	)
	if err != nil {
		panic(err)
	}
	return p
}

func (bt *beater) client(insecure bool) (string, *http.Client) {
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	bt.mutex.Lock() // for reading bt.server
	defer bt.mutex.Unlock()
	if parsed, err := url.Parse(bt.server.Addr); err == nil && parsed.Scheme == "unix" {
		transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", parsed.Path)
		}
		return "http://test-apm-server/", &http.Client{
			Transport: transport,
		}
	}
	scheme := "http://"
	if bt.config.TLS.IsEnabled() {
		scheme = "https://"
	}
	return scheme + bt.config.Host, &http.Client{Transport: transport}
}

func (bt *beater) wait() error {
	wait := make(chan struct{}, 1)

	go func() {
		for {
			bt.mutex.Lock()
			if bt.server != nil {
				bt.mutex.Unlock()
				break
			}
			bt.mutex.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
		wait <- struct{}{}
	}()
	timeout := time.NewTimer(2 * time.Second)

	select {
	case <-wait:
		return nil
	case <-timeout.C:
		return errors.New("timeout waiting server create")
	}
}

func (bt *beater) sourcemapElasticsearchHosts() []string {
	var content map[string]interface{}
	if err := bt.config.RumConfig.SourceMapping.EsConfig.Unpack(&content); err != nil {
		return nil
	}
	hostsContent := content["hosts"].([]interface{})
	hosts := make([]string, len(hostsContent))
	for i := range hostsContent {
		hosts[i] = hostsContent[i].(string)
	}
	return hosts
}

func setupBeater(t *testing.T, apmBeat *beat.Beat, ucfg *common.Config, beatConfig *beat.BeatConfig) (*beater, func(), error) {
	// create our beater
	beatBeater, err := New(apmBeat, ucfg)
	assert.NoError(t, err)
	assert.NotNil(t, beatBeater)

	c := make(chan error)
	// start it
	go func() {
		err := beatBeater.Run(apmBeat)
		if err != nil {
			c <- err
		}
	}()

	btr := beatBeater.(*beater)
	if err := btr.wait(); err != nil {
		return nil, nil, err
	}

	url, client := btr.client(true)
	go func() {
		waitForServer(url, client, c)
	}()
	select {
	case err := <-c:
		return btr, beatBeater.Stop, err
	case <-time.After(time.Second * 10):
		return nil, nil, errors.New("timeout waiting for server start")
	}
}
