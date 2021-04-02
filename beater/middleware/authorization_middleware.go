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
	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

// AuthorizationMiddleware returns a Middleware to only let authorized requests pass through
func AuthorizationMiddleware(auth *authorization.Handler, required bool) Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			header := c.Request.Header.Get(headers.Authorization)
			auth := auth.AuthorizationFor(authorization.ParseAuthorizationHeader(header))

			result, err := auth.AuthorizedFor(c.Request.Context(), authorization.ResourceInternal)
			if err != nil {
				c.Result.Err = err
				return
			} else if required && !result.Authorized {
				id := request.IDResponseErrorsUnauthorized
				if err != nil {
					id = request.IDResponseErrorsServiceUnavailable
				}

				var body interface{}
				status := request.MapResultIDToStatus[id]
				if err != nil {
					body = status.Keyword
				} else if result.Reason != "" {
					status.Keyword = result.Reason
				}
				c.Result.Set(id, status.Code, status.Keyword, body, err)
				c.Write()
				return
			}
			c.AuthResult = result

			h(c)
		}, nil
	}
}
