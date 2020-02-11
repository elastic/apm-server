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

	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
)

var (
	supportedHeaders = []string{headers.ContentType, headers.ContentEncoding, headers.Accept}
	supportedMethods = strings.Join([]string{http.MethodPost, http.MethodOptions}, ", ")
)

// CORSMiddleware returns a middleware serving preflight OPTION requests and terminating requests if they do not
// match the required valid origin.
func CORSMiddleware(allowedOrigins, allowedHeaders []string) Middleware {
	var isAllowed = func(origin string) bool {
		for _, allowed := range allowedOrigins {
			if glob.Glob(allowed, origin) {
				return true
			}
		}
		return false
	}

	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			// origin header is always set by the browser
			origin := c.Request.Header.Get(headers.Origin)
			validOrigin := isAllowed(origin)

			if c.Request.Method == http.MethodOptions {
				// setting the ACAO header is the way to tell the browser to go ahead with the request
				if validOrigin {
					// do not set the configured origin(s), echo the received origin instead
					c.Header().Set(headers.AccessControlAllowOrigin, origin)
				}

				// tell browsers to cache response requestHeaders for up to 1 hour (browsers might ignore this)
				c.Header().Set(headers.AccessControlMaxAge, "3600")
				// origin must be part of the cache key so that we can handle multiple allowed origins
				c.Header().Set(headers.Vary, "Origin")

				// required if Access-Control-Request-Method and Access-Control-Request-Headers are in the requestHeaders
				c.Header().Set(headers.AccessControlAllowMethods, supportedMethods)
				h := append(allowedHeaders, supportedHeaders...)
				c.Header().Set(headers.AccessControlAllowHeaders, strings.Join(h, ", "))

				c.Header().Set(headers.AccessControlExposeHeaders, headers.Etag)

				c.Header().Set(headers.ContentLength, "0")

				c.Result.SetDefault(request.IDResponseValidOK)
				c.Write()

			} else if validOrigin {
				// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
				c.Header().Set(headers.AccessControlAllowOrigin, origin)
				h(c)

			} else {
				c.Result.SetWithError(request.IDResponseErrorsForbidden,
					errors.New("origin: '"+origin+"' is not allowed"))
				c.Write()
			}
		}, nil
	}
}
