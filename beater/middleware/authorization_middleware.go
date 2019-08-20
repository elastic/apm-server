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
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

// RequireAuthorizationMiddleware returns a Middleware to only let authorized requests pass through
func RequireAuthorizationMiddleware(token string) Middleware {
	return func(h request.Handler) request.Handler {
		return func(c *request.Context) {
			c.TokenSet = tokenSet(token)

			if !isAuthorized(c.Request, token) {
				c.Authorized = false
				c.Result.SetDefault(request.IDResponseErrorsUnauthorized)
				c.Write()
				return
			}

			c.Authorized = true
			h(c)
		}
	}
}

// SetAuthorizationMiddleware returns a middleware setting authorization information in the context without terminating the
// request if it is not authorized
func SetAuthorizationMiddleware(token string) Middleware {
	return func(h request.Handler) request.Handler {
		return func(c *request.Context) {
			c.Authorized = isAuthorized(c.Request, token)
			c.TokenSet = tokenSet(token)
			h(c)
		}
	}
}

// isAuthorized checks the Authorization header. It must be in the form of:
//   Authorization: Bearer <secret-token>
// Bearer must be part of it.
func isAuthorized(req *http.Request, token string) bool {
	// No token configured
	if !tokenSet(token) {
		return true
	}
	header := req.Header.Get(headers.Authorization)
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != headers.Bearer {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(token)) == 1
}

func tokenSet(token string) bool {
	return token != ""
}
