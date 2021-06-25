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
	"github.com/elastic/apm-server/beater/ratelimit"
	"github.com/elastic/apm-server/beater/request"
)

// AnonymousRateLimitMiddleware adds a rate.Limiter to the context of anonymous
// requests, first ensuring the client is allowed to perform a single event and
// responding with 429 Too Many Requests if it is not.
//
// This middleware must be wrapped by AuthorizationMiddleware, as it depends on
// the value of c.AuthResult.Anonymous.
func AnonymousRateLimitMiddleware(store *ratelimit.Store) Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			if c.AuthResult.Anonymous {
				limiter := store.ForIP(c.SourceIP)
				if !limiter.Allow() {
					c.Result.SetWithError(
						request.IDResponseErrorsRateLimit,
						ratelimit.ErrRateLimitExceeded,
					)
					c.Write()
					return
				}
				ctx := c.Request.Context()
				ctx = ratelimit.ContextWithLimiter(ctx, limiter)
				c.Request = c.Request.WithContext(ctx)
			}
			h(c)
		}, nil
	}
}
