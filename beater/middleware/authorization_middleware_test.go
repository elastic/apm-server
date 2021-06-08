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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

func TestAuthorizationMiddleware(t *testing.T) {

	for name, tc := range map[string]struct {
		header             string
		securedResult      authorization.Result
		allowedWhenSecured bool
	}{
		"no header": {
			securedResult: authorization.Result{
				Authorized: false,
				Reason:     "missing or improperly formatted Authorization header: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'",
			},
		},
		"invalid header": {
			header: "Foo Bar",
			securedResult: authorization.Result{
				Authorized: false,
				Reason:     "unknown Authorization kind Foo: expected 'Authorization: Bearer secret_token' or 'Authorization: ApiKey base64(API key ID:API key)'",
			},
		},
		"invalid token": {
			header: "Bearer Bar",
			securedResult: authorization.Result{
				Authorized: false,
			},
		},
		"bearer": {
			header:             "Bearer foo",
			allowedWhenSecured: true,
			securedResult:      authorization.Result{Authorized: true},
		},
	} {
		setup := func(token string) (*authorization.Handler, *request.Context, *httptest.ResponseRecorder) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			if tc.header != "" {
				c.Request.Header.Set(headers.Authorization, tc.header)
			}
			builder, err := authorization.NewBuilder(&config.Config{SecretToken: token})
			require.NoError(t, err)
			return builder.ForAnyOfPrivileges(authorization.ActionAny), c, rec
		}

		t.Run(name+"secured required", func(t *testing.T) {
			handler, c, rec := setup("foo")
			m := AuthorizationMiddleware(handler, true)
			Apply(m, beatertest.Handler202)(c)
			if tc.allowedWhenSecured {
				assert.Equal(t, http.StatusAccepted, rec.Code)
			} else {
				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				// response body should be something like `{"error":"unauthorized"}`
				reason := tc.securedResult.Reason
				if reason == "" {
					reason = "unauthorized"
				}
				expected, err := json.Marshal(map[string]interface{}{"error": reason})
				require.NoError(t, err)
				assert.Equal(t, string(expected)+"\n", rec.Body.String())
			}
		})

		t.Run(name+"secured", func(t *testing.T) {
			handler, c, rec := setup("foo")
			m := AuthorizationMiddleware(handler, false)
			Apply(m, beatertest.Handler202)(c)
			assert.Equal(t, http.StatusAccepted, rec.Code)
			assert.Equal(t, tc.securedResult, c.AuthResult)
		})

		t.Run(name+"unsecured required", func(t *testing.T) {
			handler, c, rec := setup("")
			m := AuthorizationMiddleware(handler, true)
			Apply(m, beatertest.Handler202)(c)
			assert.Equal(t, http.StatusAccepted, rec.Code)
			assert.Equal(t, authorization.Result{Authorized: true}, c.AuthResult)
		})

		t.Run(name+"unsecured", func(t *testing.T) {
			handler, c, rec := setup("")
			m := AuthorizationMiddleware(handler, false)
			Apply(m, beatertest.Handler202)(c)
			assert.Equal(t, http.StatusAccepted, rec.Code)
			assert.Equal(t, authorization.Result{Authorized: true}, c.AuthResult)
		})
	}
}

func TestAuthorizationMiddlewareError(t *testing.T) {
	auth := authorizationFunc(func(ctx context.Context, resource authorization.Resource) (authorization.Result, error) {
		return authorization.Result{Authorized: true}, errors.New("internal details should not be leaked")
	})
	handler := authorizationHandlerFunc(func(kind, value string) authorization.Authorization {
		return auth
	})
	for _, required := range []bool{false, true} {
		c, rec := beatertest.DefaultContextWithResponseRecorder()
		m := AuthorizationMiddleware(handler, required)
		Apply(m, beatertest.Handler202)(c)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, `{"error":"service unavailable"}`+"\n", rec.Body.String())
		assert.EqualError(t, c.Result.Err, "internal details should not be leaked")
		assert.Zero(t, c.AuthResult)
	}
}

type authorizationHandlerFunc func(kind, value string) authorization.Authorization

func (f authorizationHandlerFunc) AuthorizationFor(kind, value string) authorization.Authorization {
	return f(kind, value)
}

type authorizationFunc func(context.Context, authorization.Resource) (authorization.Result, error)

func (f authorizationFunc) AuthorizedFor(ctx context.Context, resource authorization.Resource) (authorization.Result, error) {
	return f(ctx, resource)
}
