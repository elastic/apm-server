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

package apmservertest_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/systemtest/apmservertest"
)

func TestAPMServer(t *testing.T) {
	srv := apmservertest.NewServer(t)
	require.NotNil(t, srv)
	err := srv.Close()
	assert.NoError(t, err)
}

func TestUnstartedAPMServer(t *testing.T) {
	srv := apmservertest.NewUnstartedServer(t)
	err := srv.Close()
	require.Error(t, err)
	assert.EqualError(t, err, "apm-server not started")
}

func TestAPMServerStartTLS(t *testing.T) {
	srv := apmservertest.NewUnstartedServer(t)
	require.NotNil(t, srv)
	err := srv.StartTLS()
	assert.NoError(t, err)

	serverURL, err := url.Parse(srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "https", serverURL.Scheme)

	// Make sure the Tracer is configured with the
	// appropriate CA certificate.
	tracer := srv.Tracer()
	tracer.StartTransaction("name", "type").End()
	tracer.Flush(nil)
	assert.Zero(t, tracer.Stats().Errors)

	err = srv.Close()
	assert.NoError(t, err)
}

func TestExpvar(t *testing.T) {
	srv := apmservertest.NewServer(t)
	expvar := srv.GetExpvar()
	require.NotNil(t, expvar)
	assert.NotZero(t, expvar.Cmdline)
	assert.NotZero(t, expvar.Memstats)
	assert.NotEmpty(t, expvar.Vars)
}
