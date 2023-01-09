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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apmVersion "github.com/elastic/apm-server/internal/version"
	esv8 "github.com/elastic/go-elasticsearch/v8"
)

func TestClient(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		goESClient, err := NewClient(nil)
		assert.Error(t, err)
		assert.Nil(t, goESClient)
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := Config{Hosts: Hosts{"localhost:9200", "localhost:9201"}}
		goESClient, err := NewClient(&cfg)
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
	client, err := NewClient(&cfg)
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
	client, err := NewClient(&cfg)
	require.NoError(t, err)

	CreateAPIKey(context.Background(), client, CreateAPIKeyRequest{})
	select {
	case <-wait:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out while waiting for request")
	}
}
