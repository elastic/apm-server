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

package kibana

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-agent-libs/kibana"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient(kibana.ClientConfig{
		Host: "testing.invalid",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestClientAuthorization(t *testing.T) {
	test := func(username, password, apiKey, expectedAuthorization string) {
		var headers http.Header
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headers = r.Header
		}))
		defer srv.Close()

		client, err := NewClient(kibana.ClientConfig{
			Host:     srv.URL,
			APIKey:   apiKey,
			Username: username,
			Password: password,
		})
		require.NoError(t, err)

		resp, err := client.Send(context.Background(), http.MethodGet, "", nil, nil, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, expectedAuthorization, headers.Get("Authorization"))
	}
	test("", "", "foo-id:bar-apikey", "ApiKey Zm9vLWlkOmJhci1hcGlrZXk=")
	test("user", "pass", "", "Basic dXNlcjpwYXNz")
}

func TestClientHeaders(t *testing.T) {
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header
	}))
	defer srv.Close()

	client, err := NewClient(kibana.ClientConfig{
		Host:     srv.URL,
		Username: "user",
		Password: "pass",
		Headers: map[string]string{
			"Combined": "abc",
		},
	})
	require.NoError(t, err)

	resp, err := client.Send(
		context.Background(), http.MethodGet, "", nil, http.Header{
			"Combined": []string{"123"},
		}, nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "Basic dXNlcjpwYXNz", headers.Get("Authorization"))
	assert.Equal(t, []string{"abc", "123"}, headers.Values("Combined"))
}

func TestClientSend(t *testing.T) {
	var requestURL *url.URL
	var requestBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL
		requestBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("response_body"))
	}))
	defer srv.Close()

	client, err := NewClient(kibana.ClientConfig{
		Host: srv.URL,
		Path: "/base",
	})
	require.NoError(t, err)

	queryParams := url.Values{"query": []string{"param"}}
	resp, err := client.Send(
		context.Background(), http.MethodPost, "/extra", queryParams, nil, strings.NewReader("body"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "/base/extra", requestURL.Path)
	assert.Equal(t, queryParams, requestURL.Query())
	assert.Equal(t, "body", string(requestBody))

	responseBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "response_body", string(responseBody))
	assert.Equal(t, http.StatusTeapot, resp.StatusCode)
}
