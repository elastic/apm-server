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
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apmVersion "github.com/elastic/apm-server/internal/version"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
	esv8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func TestClient(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		goESClient, err := NewClient(nil, logptest.NewTestingLogger(t, ""))
		assert.Error(t, err)
		assert.Nil(t, goESClient)
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := Config{Hosts: Hosts{"localhost:9200", "localhost:9201"}}
		goESClient, err := NewClient(&cfg, logptest.NewTestingLogger(t, ""))
		require.NoError(t, err)
		assert.NotNil(t, goESClient)
	})
}

func TestClientCustomHeaders(t *testing.T) {
	var requestHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHeaders = r.Header
	}))
	defer srv.Close()

	cfg := Config{
		Hosts:   Hosts{srv.URL},
		Headers: map[string]string{"custom": "header"},
	}
	client, err := NewClient(&cfg, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	CreateAPIKey(context.Background(), client, CreateAPIKeyRequest{})
	assert.Equal(t, "header", requestHeaders.Get("custom"))
}

func TestClientCustomUserAgent(t *testing.T) {
	wait := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, fmt.Sprintf("Elastic-APM-Server/%s go-elasticsearch/%s", apmVersion.Version, esv8.Version), r.Header.Get("User-Agent"))
		close(wait)
	}))
	defer srv.Close()

	cfg := Config{
		Hosts: Hosts{srv.URL},
	}
	client, err := NewClient(&cfg, logptest.NewTestingLogger(t, ""))
	require.NoError(t, err)

	CreateAPIKey(context.Background(), client, CreateAPIKeyRequest{})
	select {
	case <-wait:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out while waiting for request")
	}
}

func esMockHandler(responder http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/_bulk":
			responder(w, r)
			return
		default:
			http.Error(w, "unsupported request", 419) // Signal unexpected error
			return
		}
	}
}

func TestClientRetryableStatuses(t *testing.T) {
	tests := []struct {
		name                 string
		responseStatusCode   int
		expectedStatusCode   int
		expectedRequestCount int
	}{
		{
			name:                 "retry 429 Too Many Requests",
			responseStatusCode:   http.StatusTooManyRequests,
			expectedStatusCode:   http.StatusOK,
			expectedRequestCount: 2,
		},
		{
			name:                 "retry 502 Bad Gateway",
			responseStatusCode:   http.StatusBadGateway,
			expectedStatusCode:   http.StatusBadGateway,
			expectedRequestCount: 1,
		},
		{
			name:                 "retry 503 Service Not Available",
			responseStatusCode:   http.StatusServiceUnavailable,
			expectedStatusCode:   http.StatusServiceUnavailable,
			expectedRequestCount: 1,
		},
		{
			name:                 "retry 504 Gateway Timeout",
			responseStatusCode:   http.StatusGatewayTimeout,
			expectedStatusCode:   http.StatusGatewayTimeout,
			expectedRequestCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxRetries := 2
			count := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if count < maxRetries {
					count += 1
					http.Error(w, "", tt.responseStatusCode)
					return
				}

				w.WriteHeader(http.StatusOK)
			})

			es := esMockHandler(handler)
			srv := httptest.NewServer(&es)
			defer srv.Close()

			c := Config{
				Username: "test",
				Password: "foobar",
				Backoff: BackoffConfig{
					Init: 0,
					Max:  0,
				},
				MaxRetries: maxRetries,
				Hosts:      []string{srv.URL},
			}
			client, err := NewClient(&c, logptest.NewTestingLogger(t, ""))
			require.NoError(t, err)

			var buf bytes.Buffer
			var res *esapi.Response
			res, err = client.Bulk(bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatusCode, res.StatusCode)
			assert.Equal(t, tt.expectedRequestCount, count)
		})
	}
}
