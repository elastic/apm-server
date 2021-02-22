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

package systemtest_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest"
	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestResponseHeaders(t *testing.T) {
	systemtest.CleanupElasticsearch(t)
	srv := apmservertest.NewUnstartedServer(t)
	srv.Config.ResponseHeaders = http.Header{}
	srv.Config.ResponseHeaders.Set("both", "all_value")
	srv.Config.RUM = &apmservertest.RUMConfig{Enabled: true, ResponseHeaders: http.Header{}}
	srv.Config.RUM.ResponseHeaders.Set("only_rum", "rum_value")
	srv.Config.RUM.ResponseHeaders.Set("both", "rum_value")
	err := srv.Start()
	require.NoError(t, err)

	// Non-RUM response headers are added to responses of non-RUM specific routes.
	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, []string{"all_value"}, resp.Header.Values("both"))
	assert.Nil(t, resp.Header.Values("only_rum"))

	// Both RUM and non-RUM response headers are added to responses of RUM-specific routes.
	// If the same key is defined in both, then the values are concatenated.
	resp, err = http.Get(srv.URL + "/config/v1/rum/agents")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, []string{"all_value", "rum_value"}, resp.Header.Values("both"))
	assert.Equal(t, []string{"rum_value"}, resp.Header.Values("only_rum"))
}
