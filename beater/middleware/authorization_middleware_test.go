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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/middleware/authorization"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/utility"
)

func TestAuthorizationMiddleware(t *testing.T) {
	t.Run("no means", func(t *testing.T) {
		c, rec := beatertest.DefaultContextWithResponseRecorder()
		auth := c.Authorization

		// apply: false
		m := AuthorizationMiddleware(false, nil, "")
		Apply(m, beatertest.Handler202)(c)
		assert.Equal(t, http.StatusAccepted, rec.Code)
		assert.Equal(t, auth, c.Authorization)

		// apply: true
		m = AuthorizationMiddleware(true, nil, "")
		Apply(m, beatertest.Handler202)(c)
		assert.Equal(t, http.StatusAccepted, rec.Code)
		assert.Equal(t, auth, c.Authorization)
	})
}

var testcases = map[string]struct {
	serverToken, requestToken, privilege string
	header                               string

	authorized     bool
	authConfigured bool
}{
	"noToken": {authorized: true, authConfigured: false},
	"validBearerToken": {
		serverToken: "1234", requestToken: "1234", header: headers.Bearer, authorized: true, authConfigured: true},
	"invalidBearerToken": {
		serverToken: "1234", requestToken: "xyz", header: headers.Bearer, authorized: false, authConfigured: true},
	"validToken": {
		serverToken: "1234", requestToken: "1234", privilege: "action:access", header: headers.APIKey, authorized: true, authConfigured: true},
	"invalidToken": {
		serverToken: "1234", requestToken: "xyz", privilege: "action:access", header: headers.APIKey, authorized: false, authConfigured: true},
	"invalidPrivilege": {
		serverToken: "1234", requestToken: "1234", privilege: "action:foo", header: headers.APIKey, authorized: false, authConfigured: true},
	"invalidAuthHeaderKey": {
		serverToken: "1234", requestToken: "xyz", header: "foo", authorized: false, authConfigured: true},
}

func TestAuthorizationMiddlewareApplied(t *testing.T) {
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			if tc.header != "" {
				c.Request.Header.Set(headers.Authorization, tc.header+" "+tc.requestToken)
			}
			m := AuthorizationMiddleware(true, means(tc.serverToken, tc.requestToken), tc.privilege)
			Apply(m, beatertest.Handler202)(c)

			if tc.authorized {
				assert.Equal(t, http.StatusAccepted, rec.Code)
			} else {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				body := beatertest.ResultErrWrap(request.MapResultIDToStatus[request.IDResponseErrorsUnauthorized].Keyword)
				assert.Equal(t, body, rec.Body.String())
			}

			// check context's authorization after setting in middleware
			authorized, _ := c.Authorization.AuthorizedFor("", tc.privilege)
			assert.Equal(t, tc.authorized, authorized)
			assert.Equal(t, tc.authConfigured, c.Authorization.IsAuthorizationConfigured())

		})
	}
}

func TestAuthorizationMiddlewareNotApplied(t *testing.T) {
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			if tc.header != "" {
				c.Request.Header.Set(headers.Authorization, tc.header+" "+tc.requestToken)
			}
			m := AuthorizationMiddleware(false, means(tc.serverToken, tc.requestToken), tc.privilege)
			Apply(m, beatertest.Handler202)(c)

			assert.Equal(t, http.StatusAccepted, rec.Code)

			// check context's authorization after setting in middleware
			authorized, _ := c.Authorization.AuthorizedFor("", tc.privilege)
			assert.Equal(t, tc.authorized, authorized)
			assert.Equal(t, tc.authConfigured, c.Authorization.IsAuthorizationConfigured())
		})
	}
}

type testAuth struct {
	authConfigured  bool
	validToken      bool
	validPrivileges []string
}

func (a *testAuth) AuthorizedFor(app string, privilege string) (bool, error) {
	if privilege == "error" {
		return false, errors.New("some error")
	}
	return a.validToken && utility.Contains(privilege, a.validPrivileges), nil
}
func (a *testAuth) IsAuthorizationConfigured() bool {
	return a.authConfigured
}

func means(serverToken, requestToken string) AuthMeans {
	return AuthMeans{
		headers.Bearer: func(token string) request.Authorization {
			return authorization.NewBearer(serverToken, token)
		},
		headers.APIKey: func(token string) request.Authorization {
			return &testAuth{authConfigured: true, validToken: serverToken == requestToken,
				validPrivileges: []string{"action:access"}}
		},
	}
}
