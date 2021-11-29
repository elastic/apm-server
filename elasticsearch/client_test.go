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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/version"
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

	t.Run("valid version", func(t *testing.T) {
		cfg := Config{Hosts: Hosts{"localhost:9200", "localhost:9201"}}
		goESClient, err := NewClient(&cfg)
		require.NoError(t, err)
		if strings.HasPrefix(version.GetDefaultVersion(), "8.") {
			_, ok := goESClient.(clientV8)
			assert.True(t, ok)
		} else if strings.HasPrefix(version.GetDefaultVersion(), "7.") {
			_, ok := goESClient.(clientV7)
			assert.True(t, ok)
		} else {
			assert.Fail(t, "unknown version ", version.GetDefaultVersion())
		}
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
	assert.Equal(t, "", requestHeaders.Get("Accept"))
}

func TestCompatibility(t *testing.T) {
	var requestHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHeaders = r.Header
	}))
	defer srv.Close()

	// compatibility header is set
	cfg := Config{
		Hosts:   Hosts{srv.URL},
		Headers: map[string]string{"Content-Type": "application/vnd.elasticsearch+json; compatible-with=7"},
	}

	client, err := NewClient(&cfg)
	require.NoError(t, err)

	CreateAPIKey(context.Background(), client, CreateAPIKeyRequest{})
	// The go-elasticsearch library takes care of setting
	assert.Equal(t, "application/vnd.elasticsearch+json;compatible-with=7", requestHeaders.Get("Accept"))
}
