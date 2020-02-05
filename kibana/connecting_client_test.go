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
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/kibana"

	"github.com/elastic/apm-server/convert"
)

func TestNewConnectingClientFrom(t *testing.T) {
	c := NewConnectingClient(mockCfg)
	require.NotNil(t, c)
	assert.Nil(t, c.(*ConnectingClient).client)
	assert.Equal(t, mockCfg, c.(*ConnectingClient).cfg)
}

func TestConnectingClient_Send(t *testing.T) {
	t.Run("Send", func(t *testing.T) {
		c := mockClient()
		r, err := c.Send(http.MethodGet, "", nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, mockBody, r.Body)
		assert.Equal(t, mockStatus, r.StatusCode)
	})

	t.Run("SendError", func(t *testing.T) {
		c := NewConnectingClient(mockCfg)
		r, err := c.Send(http.MethodGet, "", nil, nil, nil)
		require.Error(t, err)
		assert.Equal(t, err, errNotConnected)
		assert.Nil(t, r)
	})
}

func TestConnectingClient_GetVersion(t *testing.T) {
	t.Run("GetVersion", func(t *testing.T) {
		c := mockClient()
		v, err := c.GetVersion()
		require.NoError(t, err)
		assert.Equal(t, mockVersion, v)
	})

	t.Run("GetVersionError", func(t *testing.T) {
		c := NewConnectingClient(mockCfg)
		v, err := c.GetVersion()
		require.Error(t, err)
		assert.Equal(t, err, errNotConnected)
		assert.Equal(t, common.Version{}, v)
	})
}

func TestConnectingClient_SupportsVersion(t *testing.T) {
	t.Run("SupportsVersionTrue", func(t *testing.T) {
		c := mockClient()
		s, err := c.SupportsVersion(common.MustNewVersion("7.3.0"), false)
		require.NoError(t, err)
		assert.True(t, s)
	})
	t.Run("SupportsVersionFalse", func(t *testing.T) {
		c := mockClient()
		s, err := c.SupportsVersion(common.MustNewVersion("7.4.0"), false)
		require.NoError(t, err)
		assert.False(t, s)
	})

	t.Run("SupportsVersionError", func(t *testing.T) {
		c := NewConnectingClient(mockCfg)
		s, err := c.SupportsVersion(common.MustNewVersion("7.3.0"), false)
		require.Error(t, err)
		assert.Equal(t, err, errNotConnected)
		assert.False(t, s)
	})
}

func TestConnectingClient_Connected(t *testing.T) {
	t.Run("Connected", func(t *testing.T) {
		c := mockClient()
		require.True(t, c.Connected())
	})

	t.Run("NotConnected", func(t *testing.T) {
		c := NewConnectingClient(mockCfg)
		require.False(t, c.Connected())
	})
}

type rt struct {
	resp *http.Response
}

var (
	mockCfg = &kibana.ClientConfig{
		Host: "non-existing",
	}
	mockBody    = ioutil.NopCloser(convert.ToReader(`{"response": "ok"}`))
	mockStatus  = http.StatusOK
	mockVersion = *common.MustNewVersion("7.3.0")
)

// RoundTrip implements the Round Tripper interface
func (rt rt) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.resp, nil
}
func mockClient() *ConnectingClient {
	return &ConnectingClient{client: &kibana.Client{
		Connection: kibana.Connection{
			HTTP: &http.Client{
				Transport: rt{resp: &http.Response{
					StatusCode: mockStatus,
					Body:       mockBody}},
			},
			Version: mockVersion,
		},
	}}
}
