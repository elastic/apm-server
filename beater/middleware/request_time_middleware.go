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
	"time"

	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/utility"
)

// RequestTimeMiddleware returns a Middleware setting the current time in the request's context.
func RequestTimeMiddleware() Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			c.Request = c.Request.WithContext(utility.ContextWithRequestTime(c.Request.Context(), time.Now()))
			h(c)
		}, nil
	}
}
