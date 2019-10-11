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

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/authorization"
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

func TestAuthorizationMiddlewareApplied(t *testing.T) {
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			c.Request.Header.Set(headers.Authorization, "Bearer "+tc.requestToken)
			var means AuthMeans
			if tc.serverToken != "" {
				means = AuthMeans{"Bearer": &AuthMean{
					Authorization: func(token string) request.Authorization {
						return authorization.NewBearer(tc.serverToken, tc.requestToken)
					}}}
			}

			Apply(AuthorizationMiddleware(true, means, authorization.PrivilegeIntake), beatertest.Handler202)(c)

			authorized, _ := c.Authorization.AuthorizedFor("", "")
			assert.Equal(t, tc.authorized, authorized)
			assert.Equal(t, tc.tokenSet, c.Authorization.AuthorizationRequired())

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

func TestAuthorizationMiddlewareNotApplied(t *testing.T) {
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			c.Request.Header.Set(headers.Authorization, "Bearer "+tc.requestToken)
			var means AuthMeans
			if tc.serverToken != "" {
				means = AuthMeans{"Bearer": &AuthMean{
					Authorization: func(token string) request.Authorization {
						return authorization.NewBearer(tc.serverToken, tc.requestToken)
					}}}
			}

			h, err := AuthorizationMiddleware(false, means, authorization.PrivilegeIntake)(beatertest.Handler202)
			require.NoError(t, err)
			h(c)

			assert.Equal(t, http.StatusAccepted, rec.Code)
		})
	}
}
