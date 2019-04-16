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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"github.com/ryanuber/go-glob"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	supportedHeaders = "Content-Type, Content-Encoding, Accept"
	supportedMethods = "POST, OPTIONS"
)

type serverResponse struct {
	err     error
	code    int
	counter *monitoring.Int
	body    map[string]interface{}
}

var (
	serverMetrics = monitoring.Default.NewRegistry("apm-server.server", monitoring.PublishExpvar)
	counter       = func(s string) *monitoring.Int {
		return monitoring.NewInt(serverMetrics, s)
	}
	requestCounter    = counter("request.count")
	responseCounter   = counter("response.count")
	responseErrors    = counter("response.errors.count")
	responseSuccesses = counter("response.valid.count")
	responseOk        = counter("response.valid.ok")
	responseAccepted  = counter("response.valid.accepted")

	okResponse = serverResponse{
		code:    http.StatusOK,
		counter: responseOk,
	}
	acceptedResponse = serverResponse{
		code:    http.StatusAccepted,
		counter: responseAccepted,
	}
	internalErrorCounter  = counter("response.errors.internal")
	internalErrorResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "internal error"),
			code:    http.StatusInternalServerError,
			counter: internalErrorCounter,
		}
	}
	forbiddenCounter  = counter("response.errors.forbidden")
	forbiddenResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "forbidden request"),
			code:    http.StatusForbidden,
			counter: forbiddenCounter,
		}
	}
	unauthorizedResponse = serverResponse{
		err:     errors.New("invalid token"),
		code:    http.StatusUnauthorized,
		counter: counter("response.errors.unauthorized"),
	}
	requestTooLargeCounter  = counter("response.errors.toolarge")
	requestTooLargeResponse = serverResponse{
		err:     errors.New("request body too large"),
		code:    http.StatusRequestEntityTooLarge,
		counter: requestTooLargeCounter,
	}
	decodeCounter        = counter("response.errors.decode")
	cannotDecodeResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "data decoding error"),
			code:    http.StatusBadRequest,
			counter: decodeCounter,
		}
	}
	validateCounter        = counter("response.errors.validate")
	cannotValidateResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "data validation error"),
			code:    http.StatusBadRequest,
			counter: validateCounter,
		}
	}
	rateLimitCounter    = counter("response.errors.ratelimit")
	rateLimitedResponse = serverResponse{
		err:     errors.New("too many requests"),
		code:    http.StatusTooManyRequests,
		counter: rateLimitCounter,
	}
	methodNotAllowedCounter  = counter("response.errors.method")
	methodNotAllowedResponse = serverResponse{
		err:     errors.New("only POST requests are supported"),
		code:    http.StatusMethodNotAllowed,
		counter: methodNotAllowedCounter,
	}
	fullQueueCounter  = counter("response.errors.queue")
	fullQueueResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.Wrap(err, "queue is full"),
			code:    http.StatusServiceUnavailable,
			counter: fullQueueCounter,
		}
	}
	serverShuttingDownCounter  = counter("response.errors.closed")
	serverShuttingDownResponse = func(err error) serverResponse {
		return serverResponse{
			err:     errors.New("server is shutting down"),
			code:    http.StatusServiceUnavailable,
			counter: serverShuttingDownCounter,
		}
	}
)

type reqLoggerKey struct{}

func ContextWithReqLogger(ctx context.Context, rl *logp.Logger) context.Context {
	return context.WithValue(ctx, reqLoggerKey{}, rl)
}

func logHandler(h http.Handler) http.Handler {
	logger := logp.NewLogger("request")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCounter.Inc()
		reqID, err := uuid.NewV4()
		if err != nil {
			sendStatus(w, r, internalErrorResponse(err))
		}

		reqLogger := logger.With(
			"request_id", reqID,
			"method", r.Method,
			"URL", r.URL,
			"content_length", r.ContentLength,
			"remote_address", utility.RemoteAddr(r),
			"user-agent", r.Header.Get("User-Agent"))

		lw := utility.NewRecordingResponseWriter(w)
		h.ServeHTTP(lw, r.WithContext(ContextWithReqLogger(r.Context(), reqLogger)))

		if lw.Code <= 399 {
			reqLogger.Infow("handled request", []interface{}{"response_code", lw.Code}...)
		}
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
			sendStatus(w, r, forbiddenResponse(errors.New("endpoint is disabled")))
		}
	})
}

func authHandler(secretToken string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, secretToken) {
			sendStatus(w, r, unauthorizedResponse)
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

			sendStatus(w, r, okResponse)

		} else if validOrigin {
			// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
			w.Header().Set("Access-Control-Allow-Origin", origin)
			h.ServeHTTP(w, r)

		} else {
			sendStatus(w, r, forbiddenResponse(errors.New("origin: '"+origin+"' is not allowed")))
		}
	})
}

func sendStatus(w http.ResponseWriter, r *http.Request, res serverResponse) {
	responseCounter.Inc()
	res.counter.Inc()

	body := res.body
	if res.err == nil {
		responseSuccesses.Inc()
		if res.body == nil {
			w.WriteHeader(res.code)
			return
		}
	} else {
		responseErrors.Inc()

		logger := requestLogger(r)
		body = map[string]interface{}{"error": res.err.Error()}
		logger.Errorw("error handling request", "response_code", res.code, "error", body)
	}
	send(w, r, body, res.code)
}

// requestLogger is a convenience function to retrieve the logger that was
// added to the request context by handler `logHandler``
func requestLogger(r *http.Request) *logp.Logger {
	logger, ok := r.Context().Value(reqLoggerKey{}).(*logp.Logger)
	if !ok {
		logger = logp.NewLogger("request")
	}
	return logger
}

func acceptsJSON(r *http.Request) bool {
	h := r.Header.Get("Accept")
	return strings.Contains(h, "*/*") || strings.Contains(h, "application/json")
}

func sendJSON(w http.ResponseWriter, body interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	buf, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		logp.NewLogger("response").Errorf("Error while generating a JSON error response: %v", err)
		return
	}

	w.Write(buf)
	w.Write([]byte("\n"))
}

func sendPlain(w http.ResponseWriter, body interface{}, statusCode int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)

	b, err := json.Marshal(body)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", body))
	}
	w.Write(b)
	w.Write([]byte("\n"))
}
