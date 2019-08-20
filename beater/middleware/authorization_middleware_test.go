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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

type authTestdata struct {
	serverToken, requestToken string

	authorized bool
	tokenSet   bool
}

var testcases = map[string]authTestdata{
	"noToken": {
		authorized: true, tokenSet: false},
	"validToken": {
		serverToken: "1234", requestToken: "1234", authorized: true, tokenSet: true,
	},
	"invalidToken": {
		serverToken: "1234", requestToken: "xyz", authorized: false, tokenSet: true,
	},
}

func TestRequireAuthorizationMiddleware(t *testing.T) {
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			c.Request.Header.Set(headers.Authorization, "Bearer "+tc.requestToken)
			RequireAuthorizationMiddleware(tc.serverToken)(beatertest.Handler202)(c)

			assert.Equal(t, tc.authorized, c.Authorized)
			assert.Equal(t, tc.tokenSet, c.TokenSet)

			if tc.authorized {
				assert.Equal(t, http.StatusAccepted, rec.Code)
			} else {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				body := beatertest.ResultErrWrap(request.MapResultIDToStatus[request.IDResponseErrorsUnauthorized].Keyword)
				assert.Equal(t, body, rec.Body.String())
			}
		})
	}
}

func TestSetAuthorizationMiddleware(t *testing.T) {
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			c.Request.Header.Set(headers.Authorization, "Bearer "+tc.requestToken)
			SetAuthorizationMiddleware(tc.serverToken)(beatertest.Handler202)(c)

			assert.Equal(t, tc.authorized, c.Authorized)
			assert.Equal(t, tc.tokenSet, c.TokenSet)
			assert.Equal(t, http.StatusAccepted, rec.Code)
		})
	}
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
