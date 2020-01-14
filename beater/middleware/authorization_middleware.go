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

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

// AuthorizationMiddleware returns a Middleware to only let authorized requests pass through
func AuthorizationMiddleware(auth *authorization.Handler, apply bool) Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			c.Authorization = auth.AuthorizationFor(fetchAuthHeader(c.Request))

			if apply {
				authorized, err := c.Authorization.AuthorizedFor(authorization.ResourceInternal)
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
