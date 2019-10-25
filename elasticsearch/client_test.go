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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestClient_httpProxyUrl(t *testing.T) {
	t.Run("proxy disabled", func(t *testing.T) {
		proxy, err := httpProxyURL(&Config{ProxyDisable: true})
		require.Nil(t, err)
		assert.Nil(t, proxy)
	})

	t.Run("proxy from ENV", func(t *testing.T) {
		// set env var for http proxy
		os.Setenv("HTTP_PROXY", "proxy")

		// create proxy function
		proxy, err := httpProxyURL(&Config{})
		require.Nil(t, err)
		// ensure proxy function is called and check url
		url, err := proxy(httptest.NewRequest(http.MethodGet, "http://example.com", nil))
		require.Nil(t, err)
		assert.Equal(t, "http://proxy", url.String())
	})

	t.Run("proxy from URL", func(t *testing.T) {
		// set env var for http proxy
		os.Setenv("HTTP_PROXY", "proxy")

		// create proxy function from URL without `http` prefix
		proxy, err := httpProxyURL(&Config{ProxyURL: "foo"})
		require.Nil(t, err)
		// ensure proxy function is called and check url
		url, err := proxy(httptest.NewRequest(http.MethodGet, "http://example.com/", nil))
		require.Nil(t, err)
		assert.Equal(t, "http://foo", url.String())

		// create proxy function from URL with `http` prefix
		proxy, err = httpProxyURL(&Config{ProxyURL: "http://foo"})
		require.Nil(t, err)
		// ensure proxy function is called and check url
		url, err = proxy(httptest.NewRequest(http.MethodGet, "http://example.com/", nil))
		require.Nil(t, err)
		assert.Equal(t, "http://foo", url.String())
	})
}

func TestClient_addresses(t *testing.T) {
	t.Run("no protocol and path", func(t *testing.T) {
		addresses, err := addresses(&Config{Hosts: []string{
			"http://localhost", "http://localhost:9300", "localhost", "192.0.0.1", "192.0.0.2:8080"}})
		require.NoError(t, err)
		expected := []string{"http://localhost:9200", "http://localhost:9300",
			"http://localhost:9200", "http://192.0.0.1:9200", "http://192.0.0.2:8080"}
		assert.ElementsMatch(t, expected, addresses)
	})

	t.Run("with protocol and path", func(t *testing.T) {
		addresses, err := addresses(&Config{Protocol: "https", Path: "xyz",
			Hosts: []string{"http://localhost", "http://localhost:9300/abc",
				"localhost/abc", "192.0.0.2:8080"}})
		require.NoError(t, err)
		expected := []string{"http://localhost:9200/xyz", "http://localhost:9300/abc",
			"https://localhost:9200/abc", "https://192.0.0.2:8080/xyz"}
		assert.ElementsMatch(t, expected, addresses)
	})
}
