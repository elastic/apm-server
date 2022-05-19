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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_warmup(t *testing.T) {
	type testCase struct {
		agents int
		events []uint
	}
	cases := []testCase{
		{1, []uint{100, 1000}},
		{16, []uint{100, 1000, 10000}},
		{64, []uint{100, 1000}},
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
				require.NoError(t, err)
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
