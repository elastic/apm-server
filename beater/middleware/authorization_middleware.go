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
	"strings"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/middleware/authorization"
	"github.com/elastic/apm-server/beater/request"
)

// AuthMeans maps authorization header keywords and the configured AuthMean per authorization type.
type AuthMeans map[string]AuthMean

// AuthMean is a function type returning an instance that implements the request.Authorization interface.
type AuthMean func(string) request.Authorization

// AuthorizationMiddleware returns a Middleware to only let authorized requests pass through
func AuthorizationMiddleware(apply bool, means AuthMeans, privilege string) Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			if len(means) <= 0 { //no authorization configured, allow all requests
				h(c)
				return
			}

			mean, token := fetchAuthHeader(c.Request)
			authHandler, ok := means[mean]
			if !ok {
				authHandler = func(string) request.Authorization { return &authorization.Deny{} }
			}
			c.Authorization = authHandler(token)

			if apply {
				authorized, err := c.Authorization.AuthorizedFor("", privilege)
				if !authorized {
					c.Result.SetDeniedAuthorization(err)
					c.Write()
					return
				}
			}

			h(c)
		}, nil
	}
}

func fetchAuthHeader(req *http.Request) (string, string) {
	header := req.Header.Get(headers.Authorization)
	parts := strings.Split(header, " ")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func hasNoAuthHeader(mean, token string) bool {
	return mean == "" && token == ""
}
