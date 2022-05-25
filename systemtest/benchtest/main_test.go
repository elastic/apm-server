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

package benchtest

import (
	"bufio"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/apm-server/systemtest/benchtest/expvar"
	"github.com/stretchr/testify/assert"
)

func Test_warmup(t *testing.T) {
	type testCase struct {
		agents int
		events []uint
	}
	cases := []testCase{
		{1, []uint{100, 1000}},
	}
	for _, c := range cases {
		for _, events := range c.events {
			t.Run(fmt.Sprintf("%d_agent_%v_events", c.agents, events), func(t *testing.T) {
				var received uint64
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/debug/vars" {
						// Report idle APM Server.
						w.Write([]byte(`{"libbeat.output.events.active":0}`))
					}

					if !strings.HasPrefix(r.URL.Path, "/intake") {
						return
					}

					var reader io.Reader
					switch r.Header.Get("Content-Encoding") {
					case "deflate":
						zreader, err := zlib.NewReader(r.Body)
						if err != nil {
							http.Error(w, fmt.Sprintf("zlib.NewReader(): %v", err), 400)
							return
						}
						defer zreader.Close()
						reader = zreader
					default:
						reader = r.Body
					}
					scanner := bufio.NewScanner(reader)
					var localReceive uint64
					var readMeta bool
					for scanner.Scan() {
						if readMeta {
							localReceive++
						} else {
							readMeta = true
						}
					}
					atomic.AddUint64(&received, localReceive)
					w.WriteHeader(http.StatusAccepted)
				}))
				defer srv.Close()
				err := warmup(c.agents, events, srv.URL, "")
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, received, uint64(events))
			})
		}
	}
}

func Test_warmupTimeout(t *testing.T) {
	type args struct {
		ingestRate float64
		events     uint
		epm        int
		agents     int
		cpus       int
	}
	tests := []struct {
		name     string
		args     args
		expected time.Duration
	}{
		{
			name: "default warmup",
			args: args{
				ingestRate: 1000,
				events:     5000,
				agents:     1,
				cpus:       16,
			},
			expected: 15 * time.Second,
		},
		{
			name: "default warmup",
			args: args{
				ingestRate: 1000,
				events:     100,
				agents:     16,
				cpus:       2,
			},
			expected: 128 * time.Second,
		},
		{
			name: "5000 events 500 agents",
			args: args{
				ingestRate: 1000,
				events:     5000,
				agents:     500,
				cpus:       16,
			},
			expected: 78125 * time.Second,
		},
		{
			name: "5000 events 500 agents with 8 cpus",
			args: args{
				ingestRate: 1000,
				events:     5000,
				agents:     500,
				cpus:       8,
			},
			expected: 156250 * time.Second,
		},
		{
			name: "50k events",
			args: args{
				ingestRate: 1000,
				events:     50000,
				agents:     1,
				cpus:       16,
			},
			expected: 50 * time.Second,
		},
		{
			name: "default events 16 agents",
			args: args{
				ingestRate: 1000,
				events:     5000,
				agents:     16,
				cpus:       16,
			},
			expected: 80 * time.Second,
		},
		{
			name: "default events 32 agents",
			args: args{
				ingestRate: 1000,
				events:     5000,
				agents:     32,
				cpus:       16,
			},
			expected: 320 * time.Second,
		},
		{
			name: "default events 12000 epm (200/s) yields a bigger timeout",
			args: args{
				ingestRate: 1000,
				events:     5000,
				epm:        200 * 60,
				agents:     32,
				cpus:       16,
			},
			expected: 1600 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := warmupTimeout(
				tt.args.ingestRate,
				tt.args.events,
				tt.args.epm,
				tt.args.agents,
				tt.args.cpus,
			)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAddExpvarMetrics(t *testing.T) {
	tests := []struct {
		name            string
		detailed        bool
		responseMetrics []string
		memstatsMetrics []string
		expectedResult  map[string]float64
	}{
		{
			name:     "with false detailed flag and no error resp",
			detailed: false,
			responseMetrics: []string{
				`"libbeat.output.events.total": 10`,
				`"beat.runtime.goroutines": 4`,
				`"beat.memstats.rss": 1048576`,
				`"apm-server.otlp.grpc.metrics.response.errors.count": 0`,
			},
			expectedResult: map[string]float64{
				"events/sec": 10,
			},
		},
		{
			name:     "with false detailed flag and error resp",
			detailed: false,
			responseMetrics: []string{
				`"libbeat.output.events.total": 10`,
				`"beat.runtime.goroutines": 4`,
				`"beat.memstats.rss": 1048576`,
				`"apm-server.otlp.grpc.metrics.response.errors.count": 1`,
			},
			expectedResult: map[string]float64{
				"events/sec":          10,
				"error_responses/sec": 1,
			},
		},
		{
			name:     "with true detailed flag and error resp",
			detailed: true,
			responseMetrics: []string{
				`"libbeat.output.events.total": 24`,
				`"apm-server.processor.transaction.transformations": 7`,
				`"apm-server.processor.span.transformations": 5`,
				`"apm-server.processor.metric.transformations": 9`,
				`"apm-server.processor.error.transformations": 3`,
				`"beat.runtime.goroutines": 4`,
				`"beat.memstats.rss": 1048576`,
				`"output.elasticsearch.bulk_requests.available": 0`,
				`"apm-server.otlp.grpc.metrics.response.errors.count": 1`,
			},
			memstatsMetrics: []string{
				`"Alloc": 10240`,
				`"NumGC": 10`,
				`"HeapAlloc": 10240`,
				`"HeapObjects": 102`,
			},
			expectedResult: map[string]float64{
				"events/sec":              24,
				"txs/sec":                 7,
				"spans/sec":               5,
				"metrics/sec":             9,
				"errors/sec":              3,
				"gc_cycles":               10,
				"max_rss":                 1048576,
				"max_goroutines":          4,
				"max_heap_alloc":          10240,
				"max_heap_objects":        102,
				"mean_available_indexers": 0,
				"error_responses/sec":     1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := getTestServer(t, tt.responseMetrics, tt.memstatsMetrics)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			collector, err := expvar.StartNewCollector(ctx, server.URL, 10*time.Millisecond)
			assert.NoError(t, err)
			<-time.After(100 * time.Millisecond)
			cancel()

			r := testing.BenchmarkResult{
				Extra: make(map[string]float64),
				T:     time.Second,
			}
			addExpvarMetrics(&r, collector, tt.detailed)

			assert.Equal(t, len(tt.expectedResult), len(r.Extra))
			for k, v := range tt.expectedResult {
				assert.Equal(t, v, r.Extra[k])
			}
		})
	}
}

// first response is always empty, second response has responseMetrics
func getTestServer(t *testing.T, responseMetrics, memstats []string) *httptest.Server {
	var count int64
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/debug/vars" {
				t.Errorf("unexpcted path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			response := "{}"
			// 12 comes from the repeated calls made while querying expvar
			if count > 12 {
				response = createResponse(responseMetrics, memstats)
			}
			w.Write([]byte(response))
			atomic.AddInt64(&count, 1)
		}),
	)
}

func createResponse(metrics, memstats []string) string {
	var resp strings.Builder
	resp.WriteByte('{')

	for i, m := range metrics {
		if i > 0 {
			resp.WriteByte(',')
		}
		resp.WriteString(m)
	}

	if len(memstats) > 0 {
		resp.WriteByte(',')
		resp.WriteString(`"memstats":{`)
		for i, m := range memstats {
			if i > 0 {
				resp.WriteByte(',')
			}
			resp.WriteString(m)
		}
		resp.WriteByte('}')
	}

	resp.WriteByte('}')

	return resp.String()
}
