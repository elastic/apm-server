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

package systemtest_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
	"github.com/elastic/apm-server/systemtest/estest"
)

func TestAPMServerInstrumentation(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.Instrumentation = &apmservertest.InstrumentationConfig{Enabled: true}
	err := srv.Start()
	require.NoError(t, err)

	// Send a transaction to the server, causing the server to
	// trace the request from the agent.
	tracer := srv.Tracer()
	tracer.StartTransaction("name", "type").End()
	tracer.Flush(nil)

	result := systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*", estest.BoolQuery{
		Filter: []interface{}{
			estest.TermQuery{
				Field: "processor.event",
				Value: "transaction",
			},
			estest.TermQuery{
				Field: "service.name",
				Value: "apm-server",
			},
			estest.TermQuery{
				Field: "transaction.type",
				Value: "request",
			},
		},
	})

	var transactionDoc struct {
		Trace       struct{ ID string }
		Transaction struct{ ID string }
	}
	err = json.Unmarshal([]byte(result.Hits.Hits[0].RawSource), &transactionDoc)
	require.NoError(t, err)
	require.NotZero(t, transactionDoc.Trace.ID)
	require.NotZero(t, transactionDoc.Transaction.ID)

	// There should be a corresponding log record with matching
	// trace.id and transaction.id, which enables trace/log correlation.
	logs := srv.Logs.Iterator()
	defer logs.Close()
	for entry := range logs.C() {
		traceID, ok := entry.Fields["trace.id"]
		if !ok {
			continue
		}
		assert.Equal(t, transactionDoc.Trace.ID, traceID)
		assert.Equal(t, transactionDoc.Transaction.ID, entry.Fields["transaction.id"])
		return
	}
	t.Fatal("failed to identify log message with matching trace IDs")
}

func TestAPMServerInstrumentationAuth(t *testing.T) {
	test := func(t *testing.T, external, useSecretToken, useAPIKey bool) {
		systemtest.CleanupElasticsearch(t)
		srv := apmservertest.NewUnstartedServer(t)
		srv.Config.AgentAuth.SecretToken = "hunter2"
		srv.Config.AgentAuth.APIKey = &apmservertest.APIKeyAuthConfig{Enabled: true}
		srv.Config.Instrumentation = &apmservertest.InstrumentationConfig{Enabled: true}

		serverURLChan := make(chan string, 1)
		if external {
			// The server URL is not known ahead of time, so we run
			// a reverse proxy which waits for the server URL.
			var serverURL string
			var serverURLOnce sync.Once
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serverURLOnce.Do(func() {
					select {
					case <-r.Context().Done():
					case serverURL = <-serverURLChan:
					}
				})
				u, err := url.Parse(serverURL)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				rp := httputil.NewSingleHostReverseProxy(u)
				rp.ServeHTTP(w, r)
			}))
			defer proxy.Close()
			srv.Config.Instrumentation.Hosts = []string{proxy.URL}
		}
		if useSecretToken {
			srv.Config.Instrumentation.SecretToken = srv.Config.AgentAuth.SecretToken
		}
		if useAPIKey {
			systemtest.InvalidateAPIKeys(t)
			defer systemtest.InvalidateAPIKeys(t)

			cmd := apiKeyCommand("create", "--name", t.Name(), "--json")
			out, err := cmd.CombinedOutput()
			require.NoError(t, err)
			attrs := decodeJSONMap(t, bytes.NewReader(out))
			srv.Config.Instrumentation.APIKey = attrs["credentials"].(string)
		}

		err := srv.Start()
		require.NoError(t, err)
		serverURLChan <- srv.URL

		// Send a transaction to the server, causing the server to
		// trace the request from the agent.
		tracer := srv.Tracer()
		tracer.StartTransaction("name", "type").End()
		tracer.Flush(nil)

		systemtest.Elasticsearch.ExpectDocs(t, "traces-apm*", estest.BoolQuery{
			Filter: []interface{}{
				estest.TermQuery{
					Field: "processor.event",
					Value: "transaction",
				},
				estest.TermQuery{
					Field: "service.name",
					Value: "apm-server",
				},
				estest.TermQuery{
					Field: "transaction.type",
					Value: "request",
				},
			},
		})
	}
	t.Run("self_no_auth", func(t *testing.T) {
		// sending data to self, no auth specified
		test(t, false, false, false)
	})
	t.Run("external_secret_token", func(t *testing.T) {
		// sending data to external server, secret token specified
		test(t, true, true, false)
	})
	t.Run("external_api_key", func(t *testing.T) {
		// sending data to external server, API Key specified
		test(t, true, false, true)
	})
}

func TestAPMServerProfiling(t *testing.T) {
	// TODO(axw) the heap profiling test specifically is flaky. This is
	// a highly experimental feature, so disable system tests for now.
	t.Skip("flaky test: https://github.com/elastic/apm-server/issues/5322")

	test := func(t *testing.T, profilingConfig *apmservertest.ProfilingConfig, expectedMetrics []string) {
		systemtest.CleanupElasticsearch(t)
		srv := apmservertest.NewUnstartedServer(t)
		srv.Config.Instrumentation = &apmservertest.InstrumentationConfig{
			Enabled:   true,
			Profiling: profilingConfig,
		}
		err := srv.Start()
		require.NoError(t, err)

		// Generate some load to cause the server to consume resources.
		tracer := srv.Tracer()
		for i := 0; i < 1000; i++ {
			tracer.StartTransaction("name", "type").End()
		}
		tracer.Flush(nil)

		result := systemtest.Elasticsearch.ExpectDocs(t, "metrics-apm.profiling*", estest.TermQuery{
			Field: "processor.event",
			Value: "profile",
		})
		assert.Equal(t, expectedMetrics, profileMetricNames(result))
	}
	t.Run("cpu", func(t *testing.T) {
		test(t, &apmservertest.ProfilingConfig{
			CPU: &apmservertest.CPUProfilingConfig{
				Enabled:  true,
				Interval: time.Second,
				Duration: time.Second,
			},
		}, []string{"cpu.ns", "duration", "samples.count"})
	})
	t.Run("heap", func(t *testing.T) {
		test(t, &apmservertest.ProfilingConfig{
			Heap: &apmservertest.HeapProfilingConfig{
				Enabled:  true,
				Interval: time.Second,
			},
		}, []string{
			"alloc_objects.count",
			"alloc_space.bytes",
			"inuse_objects.count",
			"inuse_space.bytes",
		})
	})
}

func profileMetricNames(result estest.SearchResult) []string {
	unique := make(map[string]struct{})
	var metricNames []string
	for _, hit := range result.Hits.Hits {
		profileField, ok := hit.Source["profile"].(map[string]interface{})
		if !ok {
			continue
		}
		for k, v := range profileField {
			if _, ok := v.(float64); !ok {
				continue
			}
			if _, ok := unique[k]; ok {
				continue
			}
			unique[k] = struct{}{}
			metricNames = append(metricNames, k)
		}
	}
	sort.Strings(metricNames)
	return metricNames
}
