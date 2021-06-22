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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/api/ratelimit"
	"github.com/elastic/apm-server/beater/request"
)

func TestAnonymousRateLimitMiddleware(t *testing.T) {
	type test struct {
		burst     int
		anonymous bool

		expectStatusCode int
		expectAllow      bool
	}
	for _, test := range []test{{
		burst:            0,
		anonymous:        false,
		expectStatusCode: http.StatusOK,
	}, {
		burst:            0,
		anonymous:        true,
		expectStatusCode: http.StatusTooManyRequests,
	}, {
		burst:            1,
		anonymous:        true,
		expectStatusCode: http.StatusOK,
		expectAllow:      false,
	}, {
		burst:            2,
		anonymous:        true,
		expectStatusCode: http.StatusOK,
		expectAllow:      true,
	}} {
		store, _ := ratelimit.NewStore(1, 1, test.burst)
		middleware := AnonymousRateLimitMiddleware(store)
		handler := func(c *request.Context) {
			limiter, ok := ratelimit.FromContext(c.Request.Context())
			if test.anonymous {
				require.True(t, ok)
				assert.Equal(t, test.expectAllow, limiter.Allow())
			} else {
				require.False(t, ok)
			}
		}
		wrapped, err := middleware(handler)
		require.NoError(t, err)

		c := request.NewContext()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		c.Reset(w, r)
		c.AuthResult.Anonymous = test.anonymous

		wrapped(c)
		assert.Equal(t, test.expectStatusCode, w.Code)
	}
}

func TestAnonymousRateLimitMiddlewareForIP(t *testing.T) {
	store, _ := ratelimit.NewStore(2, 1, 1)
	middleware := AnonymousRateLimitMiddleware(store)
	handler := func(c *request.Context) {}
	wrapped, err := middleware(handler)
	require.NoError(t, err)

	requestWithIP := func(ip string) int {
		c := request.NewContext()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = ip
		c.Reset(w, r)
		c.AuthResult.Anonymous = true
		wrapped(c)
		return w.Code
	}
	assert.Equal(t, http.StatusOK, requestWithIP("10.1.1.1"))
	assert.Equal(t, http.StatusTooManyRequests, requestWithIP("10.1.1.1"))
	assert.Equal(t, http.StatusOK, requestWithIP("10.1.1.2"))

	// ratelimit.Store size is 2: the 3rd IP reuses an existing (depleted) rate limiter.
	assert.Equal(t, http.StatusTooManyRequests, requestWithIP("10.1.1.3"))
}
