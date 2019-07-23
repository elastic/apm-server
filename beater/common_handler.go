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

package beater

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	"github.com/elastic/apm-server/utility"
)

var (
	supportedHeaders = fmt.Sprintf("%s, %s, %s", headers.ContentType, headers.ContentEncoding, headers.Accept)
	supportedMethods = fmt.Sprintf("%s, %s", http.MethodPost, http.MethodOptions)
)

func requestTimeHandler() middleware {
	return func(h request.Handler) request.Handler {
		return func(c *request.Context) {
			c.Request = c.Request.WithContext(utility.ContextWithRequestTime(c.Request.Context(), time.Now()))
			h(c)
		}
	}
}

func killSwitchHandler(on bool) middleware {
	return func(h request.Handler) request.Handler {
		return func(c *request.Context) {
			if on {
				h(c)
			} else {
				sendStatus(c, request.ForbiddenResponse(errors.New("endpoint is disabled")))
			}
		}
	}
}

func authHandler(secretToken string) middleware {
	return func(h request.Handler) request.Handler {
		return func(c *request.Context) {
			if !isAuthorized(c.Request, secretToken) {
				sendStatus(c, request.UnauthorizedResponse)
				return
			}
			h(c)
		}
	}
}

// isAuthorized checks the Authorization header. It must be in the form of:
//   Authorization: Bearer <secret-token>
// Bearer must be part of it.
func isAuthorized(req *http.Request, secretToken string) bool {
	// No token configured
	if secretToken == "" {
		return true
	}
	header := req.Header.Get(headers.Authorization)
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != headers.Bearer {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(secretToken)) == 1
}

func corsHandler(allowedOrigins []string) middleware {
	return func(h request.Handler) request.Handler {

		var isAllowed = func(origin string) bool {
			for _, allowed := range allowedOrigins {
				if glob.Glob(allowed, origin) {
					return true
				}
			}
			return false
		}

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
				c.Header().Set(headers.AccessControlAllowHeaders, supportedHeaders)

				c.Header().Set(headers.ContentLength, "0")

				sendStatus(c, request.OKResponse)

			} else if validOrigin {
				// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
				c.Header().Set(headers.AccessControlAllowOrigin, origin)
				h(c)

			} else {
				sendStatus(c, request.ForbiddenResponse(errors.New("origin: '"+origin+"' is not allowed")))
			}
		}
	}
}

//TODO: move to Context when reworking response handling.
func sendStatus(c *request.Context, res request.Result) {
	if res.Err != nil {
		body := map[string]interface{}{"error": res.Err.Error()}
		//TODO: refactor response handling: get rid of additional `error` and just pass in error
		c.SendError(body, body, res.StatusCode)
		return
	}

	if res.Body == nil {
		c.WriteHeader(res.StatusCode)
		return
	}

	c.Send(res.Body, res.StatusCode)
}
