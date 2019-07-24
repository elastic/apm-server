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

package beater

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/request"
)

func TestOPTIONS(t *testing.T) {
	requestTaken := make(chan struct{}, 1)
	done := make(chan struct{}, 1)

	h := WithMiddleware(
		func(c *request.Context) {
			requestTaken <- struct{}{}
			<-done
		},
		append(apmHandler(IntakeResultIDToMonitoringInt),
			KillSwitchHandler(true),
			RequestTimeHandler(),
			CorsHandler([]string{"*"}))...)

	// use this to block the single allowed concurrent requests
	go func() {
		c := &request.Context{}
		c.Reset(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
		h(c)
	}()

	<-requestTaken

	// send a new request which should be allowed through
	c := &request.Context{}
	w := httptest.NewRecorder()
	c.Reset(w, httptest.NewRequest(http.MethodOptions, "/", nil))
	h(c)

	assert.Equal(t, 200, w.Code, w.Body.String())
	done <- struct{}{}
}

func TestIsAuthorized(t *testing.T) {
	reqAuth := func(auth string) *http.Request {
		req, err := http.NewRequest(http.MethodPost, "_", nil)
		assert.Nil(t, err)
		req.Header.Add("Authorization", auth)
		return req
	}

	reqNoAuth, err := http.NewRequest(http.MethodPost, "_", nil)
	require.NoError(t, err)

	// Successes
	assert.True(t, isAuthorized(reqNoAuth, ""))
	assert.True(t, isAuthorized(reqAuth("foo"), ""))
	assert.True(t, isAuthorized(reqAuth("Bearer foo"), "foo"))

	// Failures
	assert.False(t, isAuthorized(reqNoAuth, "foo"))
	assert.False(t, isAuthorized(reqAuth("Bearer bar"), "foo"))
	assert.False(t, isAuthorized(reqAuth("Bearer foo extra"), "foo"))
	assert.False(t, isAuthorized(reqAuth("foo"), "foo"))
}
