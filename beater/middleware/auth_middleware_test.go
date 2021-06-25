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
	"fmt"
	"net/http"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

func TestAuthMiddleware(t *testing.T) {

	for name, tc := range map[string]struct {
		authHeader   string
		authRequired bool
		authError    error

		expectKind           string
		expectToken          string
		expectStatus         int
		expectBody           string
		expectAuthentication auth.AuthenticationDetails
	}{
		"authenticated": {
			authRequired: true,
			authHeader:   "Bearer abc123",
			expectKind:   "Bearer",
			expectToken:  "abc123",
			expectStatus: http.StatusAccepted,
			expectAuthentication: auth.AuthenticationDetails{
				Method: auth.MethodSecretToken,
			},
		},
		"auth_failed_required": {
			authRequired: true,
			authError:    fmt.Errorf("%w: nope", auth.ErrAuthFailed),
			expectStatus: http.StatusUnauthorized,
			expectBody:   beatertest.ResultErrWrap("authentication failed: nope"),
		},
		"auth_failed_optional": {
			authRequired: false,
			authError:    fmt.Errorf("%w: nope", auth.ErrAuthFailed),
			expectStatus: http.StatusAccepted,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c, rec := beatertest.DefaultContextWithResponseRecorder()
			if tc.authHeader != "" {
				c.Request.Header.Set(headers.Authorization, tc.authHeader)
			}
			var authenticator authenticatorFunc = func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
				assert.Equal(t, tc.expectKind, kind)
				assert.Equal(t, tc.expectToken, token)
				return auth.AuthenticationDetails{Method: auth.MethodSecretToken}, auth.AnonymousAuth{}, tc.authError
			}
			m := AuthMiddleware(authenticator, tc.authRequired)
			Apply(m, beatertest.Handler202)(c)
			assert.Equal(t, tc.expectStatus, rec.Code)
			assert.Equal(t, tc.expectBody, rec.Body.String())
			assert.Equal(t, tc.expectAuthentication, c.Authentication)
		})
	}
}

func TestAuthMiddlewareError(t *testing.T) {
	var authenticator authenticatorFunc = func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		return auth.AuthenticationDetails{}, nil, errors.New("internal details should not be leaked")
	}
	for _, required := range []bool{false, true} {
		c, rec := beatertest.DefaultContextWithResponseRecorder()
		m := AuthMiddleware(authenticator, required)
		Apply(m, beatertest.Handler202)(c)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, `{"error":"service unavailable"}`+"\n", rec.Body.String())
		assert.EqualError(t, c.Result.Err, "internal details should not be leaked")
		assert.Zero(t, c.Authentication)
	}
}

func TestAuthUnauthorized(t *testing.T) {
	var authorizer authorizerFunc = func(context.Context, auth.Action, auth.Resource) error {
		return fmt.Errorf("%w: none shall pass", auth.ErrUnauthorized)
	}
	var authenticator authenticatorFunc = func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
		return auth.AuthenticationDetails{}, authorizer, nil
	}
	next := func(c *request.Context) {
		c.Result.Err = auth.Authorize(c.Request.Context(), auth.ActionEventIngest, auth.Resource{})
	}
	c, _ := beatertest.DefaultContextWithResponseRecorder()
	m := AuthMiddleware(authenticator, true)
	Apply(m, next)(c)

	assert.Equal(t, request.IDResponseErrorsForbidden, c.Result.ID)
	assert.Equal(t, "unauthorized: none shall pass", c.Result.Body)
}

type authenticatorFunc func(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error)

func (f authenticatorFunc) Authenticate(ctx context.Context, kind, token string) (auth.AuthenticationDetails, auth.Authorizer, error) {
	return f(ctx, kind, token)
}

type authorizerFunc func(ctx context.Context, action auth.Action, resource auth.Resource) error

func (f authorizerFunc) Authorize(ctx context.Context, action auth.Action, resource auth.Resource) error {
	return f(ctx, action, resource)
}
