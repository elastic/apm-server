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
	"github.com/elastic/apm-server/beater/api/ratelimit"
	"github.com/elastic/apm-server/beater/config"
	"github.com/elastic/apm-server/beater/request"
)

const burstMultiplier = 3

// SetIPRateLimitMiddleware sets a rate limiter
func SetIPRateLimitMiddleware(cfg config.EventRate) Middleware {
	store, err := ratelimit.NewStore(cfg.LruSize, cfg.Limit, burstMultiplier)

	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			c.RateLimiter = store.ForIP(c.Request)
			h(c)
		}, err
	}
}
