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
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/elastic/beats/libbeat/monitoring"

	"github.com/elastic/apm-server/beater/internal"

	"github.com/elastic/apm-server/server"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/logp"
)

const (
	supportedHeaders = "Content-Type, Content-Encoding, Accept"
	supportedMethods = "POST, OPTIONS"
)

type reqLoggerKey struct{}

func ContextWithReqLogger(ctx context.Context, rl *logp.Logger) context.Context {
	return context.WithValue(ctx, reqLoggerKey{}, rl)
}

func logHandler(h http.Handler) http.Handler {
	logger := logp.NewLogger("request")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID, err := uuid.NewV4()
		if err != nil {
			internal.SendCnt(w, r, server.Error{err, http.StatusInternalServerError})
		}

		reqLogger := logger.With(
			"request_id", reqID,
			"method", r.Method,
			"URL", r.URL,
			"content_length", r.ContentLength,
			"remote_address", utility.RemoteAddr(r),
			"user-agent", r.Header.Get("User-Agent"))

		defer func() {
			if r := recover(); r != nil {
				var ok bool
				if err, ok = r.(error); !ok {
					err = fmt.Errorf("internal server error %+v", r)
				}
				reqLogger.Errorw("panic handling request",
					"error", err.Error(), "stacktrace", string(debug.Stack()))
			}
		}()

		lw := utility.NewRecordingResponseWriter(w)
		h.ServeHTTP(lw, r.WithContext(ContextWithReqLogger(r.Context(), reqLogger)))

		if lw.Code <= 399 {
			reqLogger.Infow("handled request", []interface{}{"code", lw.Code}...)
		} else {
			reqLogger.Errorw("error handling request", "code", lw.Code, "body", lw.Body)
		}

	})
}

var reqCounter *monitoring.Int
var once sync.Once

func requestCountHandler(h http.Handler) http.Handler {
	once.Do(func() {
		reqCounter = monitoring.NewInt(internal.ServerMetrics, "request.count")
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCounter.Inc()
		h.ServeHTTP(w, r)
	})
}

func requestTimeHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(utility.ContextWithRequestTime(r.Context(), time.Now()))
		h.ServeHTTP(w, r)
	})
}

func killSwitchHandler(killSwitch bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if killSwitch {
			h.ServeHTTP(w, r)
		} else {
			internal.SendCnt(w, r, server.Error{errors.New("endpoint is disabled"), http.StatusForbidden})
		}
	})
}

func authHandler(secretToken string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, secretToken) {
			internal.SendCnt(w, r, server.Unauthorized())
			return
		}
		h.ServeHTTP(w, r)
	})
}

// isAuthorized checks the Authorization header. It must be in the form of:
//   Authorization: Bearer <secret-token>
// Bearer must be part of it.
func isAuthorized(req *http.Request, secretToken string) bool {
	// No token configured
	if secretToken == "" {
		return true
	}
	header := req.Header.Get("Authorization")
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(secretToken)) == 1
}

func corsHandler(allowedOrigins []string, h http.Handler) http.Handler {

	var isAllowed = func(origin string) bool {
		for _, allowed := range allowedOrigins {
			if glob.Glob(allowed, origin) {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// origin header is always set by the browser
		origin := r.Header.Get("Origin")
		validOrigin := isAllowed(origin)

		if r.Method == "OPTIONS" {

			// setting the ACAO header is the way to tell the browser to go ahead with the request
			if validOrigin {
				// do not set the configured origin(s), echo the received origin instead
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			// tell browsers to cache response requestHeaders for up to 1 hour (browsers might ignore this)
			w.Header().Set("Access-Control-Max-Age", "3600")
			// origin must be part of the cache key so that we can handle multiple allowed origins
			w.Header().Set("Vary", "Origin")

			// required if Access-Control-Request-Method and Access-Control-Request-Headers are in the requestHeaders
			w.Header().Set("Access-Control-Allow-Methods", supportedMethods)
			w.Header().Set("Access-Control-Allow-Headers", supportedHeaders)

			w.Header().Set("Content-Length", "0")

			internal.Send(w, r, server.Result{StatusCode: http.StatusOK})

		} else if validOrigin {
			// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
			w.Header().Set("Access-Control-Allow-Origin", origin)
			h.ServeHTTP(w, r)

		} else {
			internal.SendCnt(w, r,
				server.Error{errors.New("origin: '" + origin + "' is not allowed"), http.StatusForbidden})
		}
	})
}
