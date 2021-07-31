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

package elasticsearch

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/outputs/elasticsearch"
)

func TestBackoffCalled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	cfg := &Config{Hosts: Hosts{srv.Listener.Addr().String()}}
	transport, addresses, headers, err := connectionConfig(cfg)
	assert.NoError(t, err)

	var (
		called  bool
		retries = 1
	)
	backoff := func(int) time.Duration {
		called = true
		return 0
	}

	c, err := NewVersionedClient(
		"",
		"",
		"",
		addresses,
		headers,
		transport,
		retries,
		backoff,
	)
	assert.NoError(t, err)

	req, err := http.NewRequest("GET", srv.URL, nil)
	assert.NoError(t, err)
	c.Perform(req)
	assert.True(t, called)
}

func TestBackoffRetries(t *testing.T) {
	var (
		requests int
		retries  = 5
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("X-Elastic-Product", "Elasticsearch")
			return
		}
		requests++
		w.WriteHeader(503)
		w.Write([]byte("error"))
	}))
	defer srv.Close()

	c, err := NewClient(&Config{
		Hosts:      Hosts{srv.URL},
		Protocol:   "http",
		Timeout:    esConnectionTimeout,
		MaxRetries: retries,
		Backoff:    elasticsearch.Backoff{},
	})
	assert.NoError(t, err)

	req, err := http.NewRequest("GET", "/some/path", nil)
	assert.NoError(t, err)
	c.Perform(req)

	// We have the initial request, and then a maximum amount of retries.
	// Add the initial request to the number of retries to get the total
	// requests we expect.
	assert.Equal(t, retries+1, requests)
}

func TestBackoffConfigured(t *testing.T) {
	init := 2 * time.Second
	backoffCfg := elasticsearch.Backoff{
		Init: init,
		Max:  time.Minute,
	}
	backoffFn := exponentialBackoff(backoffCfg)
	assert.Equal(t, init, backoffFn(1))
	assert.Equal(t, 4*time.Second, backoffFn(2))
	assert.Equal(t, 8*time.Second, backoffFn(3))
	assert.Equal(t, 16*time.Second, backoffFn(4))
	assert.Equal(t, 32*time.Second, backoffFn(5))
	assert.Equal(t, time.Minute, backoffFn(20))
}
