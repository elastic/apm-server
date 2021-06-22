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

// AuthorizationHandler provides an interface for obtaining an authorization.Authorization
// for a given auth kind and value.
type AuthorizationHandler interface {
	AuthorizationFor(kind, value string) authorization.Authorization
}

// AuthorizationMiddleware returns a Middleware to only let authorized requests pass through
func AuthorizationMiddleware(auth AuthorizationHandler, required bool) Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			header := c.Request.Header.Get(headers.Authorization)
			auth := auth.AuthorizationFor(authorization.ParseAuthorizationHeader(header))

			result, err := auth.AuthorizedFor(c.Request.Context(), authorization.Resource{})
			if err != nil {
				c.Result.SetDefault(request.IDResponseErrorsServiceUnavailable)
				c.Result.Err = err
				c.Write()
				return
			} else if required && !result.Authorized {
				id := request.IDResponseErrorsUnauthorized
				status := request.MapResultIDToStatus[id]
				if result.Reason != "" {
					status.Keyword = result.Reason
				}
				c.Result.Set(id, status.Code, status.Keyword, nil, nil)
				c.Write()
				return
			}
			c.AuthResult = result
			c.Request = c.Request.WithContext(authorization.ContextWithAuthorization(c.Request.Context(), auth))

			h(c)
		}, nil
	}
}

// AnonymousAuthorizationMiddleware returns a Middleware allowing anonymous access.
func AnonymousAuthorizationMiddleware() Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			auth := authorization.AnonymousAuth{}
			c.AuthResult = authorization.Result{Authorized: true, Anonymous: true}
			c.Request = c.Request.WithContext(authorization.ContextWithAuthorization(c.Request.Context(), auth))
			h(c)
		}, nil
	}
}
