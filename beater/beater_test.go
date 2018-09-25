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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/outputs"
	pubs "github.com/elastic/beats/libbeat/publisher"
	"github.com/elastic/beats/libbeat/publisher/pipeline"
	"github.com/elastic/beats/libbeat/publisher/queue"
	"github.com/elastic/beats/libbeat/publisher/queue/memqueue"
	"github.com/elastic/beats/libbeat/version"
)

func TestBeatConfig(t *testing.T) {
	falsy, truthy := false, true

	tests := []struct {
		conf       map[string]interface{}
		beaterConf *Config
		SmapIndex  string
		msg        string
	}{
		{
			conf:       map[string]interface{}{},
			beaterConf: defaultConfig("6.2.0"),
			msg:        "Default config created for empty config.",
		},
		{
			conf: map[string]interface{}{
				"host":                   "localhost:3000",
				"max_unzipped_size":      64,
				"max_request_queue_time": 9 * time.Second,
				"max_header_size":        8,
				"max_event_size":         100,
				"read_timeout":           3 * time.Second,
				"write_timeout":          4 * time.Second,
				"shutdown_timeout":       9 * time.Second,
				"capture_personal_data":  true,
				"secret_token":           "1234random",
				"ssl": map[string]interface{}{
					"enabled":     true,
					"key":         "1234key",
					"certificate": "1234cert",
				},
				"concurrent_requests": 15,
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"frontend": map[string]interface{}{
					"enabled":    true,
					"rate_limit": 1000,
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
				"metrics": map[string]interface{}{
					"enabled": false,
				},
				"register": map[string]interface{}{
					"ingest": map[string]interface{}{
						"pipeline": map[string]interface{}{
							"enabled":   true,
							"overwrite": false,
							"path":      filepath.Join("tmp", "definition.json"),
						},
					},
				},
			},
			beaterConf: &Config{
				Host:                "localhost:3000",
				MaxUnzippedSize:     64,
				MaxRequestQueueTime: 9 * time.Second,
				MaxHeaderSize:       8,
				MaxEventSize:        100,
				ReadTimeout:         3000000000,
				WriteTimeout:        4000000000,
				ShutdownTimeout:     9000000000,
				SecretToken:         "1234random",
				SSL:                 &SSLConfig{Enabled: &truthy, Certificate: outputs.CertificateConfig{Certificate: "1234cert", Key: "1234key"}},
				AugmentEnabled:      true,
				Expvar: &ExpvarConfig{
					Enabled: &truthy,
					Url:     "/debug/vars",
				},
				FrontendConfig: &rumConfig{
					Enabled:   &truthy,
					RateLimit: 1000,
					EventRate: &eventRate{
						Limit:   7200,
						LruSize: 2000,
					},
					AllowOrigins: []string{"example*"},
					SourceMapping: &SourceMapping{
						Cache:        &Cache{Expiration: 8 * time.Minute},
						IndexPattern: "apm-test*",
					},
					LibraryPattern:      "^custom",
					ExcludeFromGrouping: "^grouping",
					beatVersion:         "6.2.0",
				},
				RumConfig: &rumConfig{
					Enabled:   &truthy,
					RateLimit: 1000,
					EventRate: &eventRate{
						Limit:   7200,
						LruSize: 2000,
					},
					AllowOrigins: []string{"example*"},
					SourceMapping: &SourceMapping{
						Cache:        &Cache{Expiration: 8 * time.Minute},
						IndexPattern: "apm-test*",
					},
					LibraryPattern:      "^custom",
					ExcludeFromGrouping: "^grouping",
					beatVersion:         "6.2.0",
				},
				Metrics: &metricsConfig{
					Enabled: &falsy,
				},
				ConcurrentRequests: 15,
				Register: &registerConfig{
					Ingest: &ingestConfig{
						Pipeline: &pipelineConfig{
							Enabled:   &truthy,
							Overwrite: &falsy,
							Path:      filepath.Join("tmp", "definition.json"),
						},
					},
				},
			},
			msg: "Given config overwrites default",
		},
		{
			conf: map[string]interface{}{
				"host":              "localhost:3000",
				"max_unzipped_size": 64,
				"secret_token":      "1234random",
				"ssl": map[string]interface{}{
					"enabled": true,
				},
				"expvar": map[string]interface{}{
					"enabled": true,
					"url":     "/debug/vars",
				},
				"frontend": map[string]interface{}{
					"enabled":    true,
					"rate_limit": 890,
					"event_rate": map[string]interface{}{
						"lru_size": 200,
					},
					"source_mapping": map[string]interface{}{
						"cache": map[string]interface{}{
							"expiration": 4,
						},
					},
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
			beaterConf: &Config{
				Host:                "localhost:3000",
				MaxUnzippedSize:     64,
				MaxRequestQueueTime: 2 * time.Second,
				MaxHeaderSize:       1048576,
				MaxEventSize:        307200,
				ReadTimeout:         30000000000,
				WriteTimeout:        30000000000,
				ShutdownTimeout:     5000000000,
				SecretToken:         "1234random",
				SSL:                 &SSLConfig{Enabled: &truthy, Certificate: outputs.CertificateConfig{Certificate: "", Key: ""}},
				AugmentEnabled:      true,
				Expvar: &ExpvarConfig{
					Enabled: &truthy,
					Url:     "/debug/vars",
				},
				FrontendConfig: &rumConfig{
					Enabled:   &truthy,
					RateLimit: 890,
					EventRate: &eventRate{
						Limit:   300,
						LruSize: 200,
					},
					SourceMapping: &SourceMapping{
						Cache: &Cache{
							Expiration: 4 * time.Second,
						},
						IndexPattern: "apm-*-sourcemap*",
					},
					AllowOrigins:        []string{"*"},
					LibraryPattern:      "node_modules|bower_components|~",
					ExcludeFromGrouping: "^/webpack",
					beatVersion:         "6.2.0",
				},
				RumConfig: &rumConfig{
					Enabled:   &truthy,
					RateLimit: 10,
					EventRate: &eventRate{
						Limit:   300,
						LruSize: 1000,
					},
					AllowOrigins: []string{"*"},
					SourceMapping: &SourceMapping{
						Cache: &Cache{
							Expiration: 7 * time.Second,
						},
						IndexPattern: "apm-*-sourcemap*",
					},
					LibraryPattern:      "rum",
					ExcludeFromGrouping: "^/webpack",
					beatVersion:         "6.2.0",
				},
				Metrics: &metricsConfig{
					Enabled: &truthy,
				},
				ConcurrentRequests: 5,
				Register: &registerConfig{
					Ingest: &ingestConfig{
						Pipeline: &pipelineConfig{
							Enabled:   &falsy,
							Overwrite: &truthy,
							Path:      filepath.Join("ingest", "pipeline", "definition.json"),
						},
					},
				},
			},
			msg: "Given config merged with default",
		},
	}

	for _, test := range tests {
		ucfgConfig, err := common.NewConfigFrom(test.conf)
		assert.NoError(t, err)
		btr, err := New(&beat.Beat{Info: beat.Info{Version: "6.2.0"}}, ucfgConfig)
		assert.NoError(t, err)
		assert.NotNil(t, btr)
		bt := btr.(*beater)
		assert.Equal(t, test.beaterConf, bt.config, test.msg)
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

type DummyOutputClient struct {
}

func (d *DummyOutputClient) Publish(batch pubs.Batch) error {
	batch.ACK()
	return nil
}
func (d *DummyOutputClient) Close() error   { return nil }
func (d *DummyOutputClient) String() string { return "" }

func DummyPipeline(clients ...outputs.Client) *pipeline.Pipeline {
	if len(clients) == 0 {
		clients = []outputs.Client{&DummyOutputClient{}}
	}
	p, err := pipeline.New(
		beat.Info{Name: "test-apm-server"},
		pipeline.Monitors{},
		nil,
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
	if bt.config.SSL.isEnabled() {
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

func (bt *beater) smapElasticsearchHosts() []string {
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

func setupBeater(t *testing.T, publisher beat.Pipeline, ucfg *common.Config, beatConfig *beat.BeatConfig) (*beater, func(), error) {
	beatId, err := uuid.NewV4()
	require.NoError(t, err)
	// create a beat
	apmBeat := &beat.Beat{
		Publisher: publisher,
		Info: beat.Info{
			Beat:        "test-apm-server",
			IndexPrefix: "test-apm-server",
			Version:     version.GetDefaultVersion(),
			UUID:        beatId,
		},
		Config: beatConfig,
	}

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
	btr.wait()

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

func SetupServer(b *testing.B) *http.ServeMux {
	pip := DummyPipeline()
	pub, err := publish.NewPublisher(pip, 1, time.Duration(0), elasticapm.DefaultTracer)
	if err != nil {
		b.Fatal("error initializing publisher", err)
	}
	return newMuxer(defaultConfig("7.0.0"), pub.Send)
}

func pluralize(entity string) string {
	return entity + "s"
}

func createPayload(entityType string, numEntities int) []byte {
	data, err := loader.LoadValidData(entityType)
	if err != nil {
		panic(err)
	}
	var entityList []interface{}
	testEntities := data[pluralize(entityType)].([]interface{})

	for i := 0; i < numEntities; i++ {
		entityList = append(entityList, testEntities[i%len(testEntities)])
	}
	data[pluralize(entityType)] = entityList
	out, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return out
}

func benchmarkVariableSizePayload(b *testing.B, entitytype string, entries int) {
	url := "/v1/" + pluralize(entitytype)
	mux := SetupServer(b)
	data := createPayload(entitytype, entries)
	b.Logf("Using payload size: %d", len(data))
	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		req.Header.Add("Content-Type", "application/json")
		if err != nil {
			b.Error(err)
		}

		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != 202 {
			b.Fatal(w.Body.String())
		}
	}
}

func BenchmarkServer(b *testing.B) {
	entityTypes := []string{"transaction", "error"}
	sizes := []int{100, 1000, 10000}

	for _, et := range entityTypes {
		b.Run(et, func(b *testing.B) {
			for _, sz := range sizes {
				b.Run(fmt.Sprintf("size=%v", sz), func(b *testing.B) {
					benchmarkVariableSizePayload(b, et, sz)
				})
			}
		})
	}
}
