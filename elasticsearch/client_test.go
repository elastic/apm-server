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
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/version"
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

func TestMakeJSONRequest(t *testing.T) {
	var body interface{}
	req, err := makeJSONRequest(http.MethodGet, "/path", body, "Authorization:foo", "Header-X:bar")
	assert.Nil(t, err)
	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/path", req.URL.Path)
	assert.NotNil(t, req.Body)
	assert.NotNil(t, req.Header)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
	assert.Equal(t, "foo", req.Header.Get("Authorization"))
	assert.Equal(t, "bar", req.Header.Get("Header-X"))
}

func TestParseResponse(t *testing.T) {
	body := "body"
	for _, testCase := range []struct {
		code         int
		expectedBody io.ReadCloser
		expectedErr  error
	}{
		{404, nil, errors.New(body)},
		{200, ioutil.NopCloser(strings.NewReader(body)), nil},
	} {
		jsonResponse := parseResponse(&http.Response{
			StatusCode: testCase.code,
			Body:       ioutil.NopCloser(strings.NewReader(body)),
		}, nil)
		assert.Equal(t, testCase.expectedBody, jsonResponse.content)
		assert.Equal(t, testCase.expectedErr, jsonResponse.err)
	}
}

func TestDecodeTo(t *testing.T) {
	type target map[string]string
	err := errors.New("error")

	for _, testCase := range []struct {
		content        []byte
		err            error
		expectedError  error
		expectedEffect target
	}{
		{nil, err, err, target(nil)},
		{[]byte(`{"foo":"bar"}`), nil, nil, target{"foo": "bar"}},
	} {
		var to target
		assert.Equal(t, testCase.expectedError, JSONResponse{
			content: ioutil.NopCloser(bytes.NewReader(testCase.content)),
			err:     testCase.err,
		}.DecodeTo(&to))
		assert.Equal(t, testCase.expectedEffect, to)
	}
}
