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

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

// Authenticator provides an interface for authenticating a client.
type Authenticator interface {
	Authenticate(ctx context.Context, kind, value string) (auth.AuthenticationDetails, auth.Authorizer, error)
}

// AuthMiddleware returns a Middleware to authenticate clients.
//
// If required is true, then the middleware will prevent unauthenticated
// requests. Otherwise the request.Context's Authentication will be set,
// and in the case of unauthenticated requests, the Authentication field
// will have the zero value and the context will be populated with an
// auth.Authorizer that denies all actions and resources.
func AuthMiddleware(authenticator Authenticator, required bool) Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			header := c.Request.Header.Get(headers.Authorization)
			kind, token := auth.ParseAuthorizationHeader(header)
			details, authorizer, err := authenticator.Authenticate(c.Request.Context(), kind, token)
			if err != nil {
				if errors.Is(err, auth.ErrAuthFailed) {
					if !required {
						details = auth.AuthenticationDetails{}
						authorizer = denyAll{}
					} else {
						id := request.IDResponseErrorsUnauthorized
						status := request.MapResultIDToStatus[id]
						status.Keyword = err.Error()
						c.Result.Set(id, status.Code, status.Keyword, nil, nil)
						c.Write()
						return
					}
				} else {
					c.Result.SetDefault(request.IDResponseErrorsServiceUnavailable)
					c.Result.Err = err
					c.Write()
					return
				}
			}
			c.Authentication = details
			c.Request = c.Request.WithContext(auth.ContextWithAuthorizer(c.Request.Context(), authorizer))
			h(c)

			// Processors may indicate that a request is unauthorized by returning auth.ErrUnauthorized.
			if errors.Is(c.Result.Err, auth.ErrUnauthorized) {
				switch c.Result.ID {
				case request.IDUnset, request.IDResponseErrorsInternal:
					id := request.IDResponseErrorsForbidden
					status := request.MapResultIDToStatus[id]
					c.Result.Set(id, status.Code, c.Result.Keyword, c.Result.Body, c.Result.Err)
				}
			}
		}, nil
	}
}

type denyAll struct{}

func (denyAll) Authorize(context.Context, auth.Action, auth.Resource) error {
	return auth.ErrUnauthorized
}
