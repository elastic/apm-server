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

	"github.com/gofrs/uuid"

	"go.elastic.co/apm"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/utility"
)

// LogMiddleware returns a middleware taking care of logging processing a request in the middleware and the request handler
func LogMiddleware() Middleware {
	logger := logp.NewLogger(logs.Request)
	return func(h request.Handler) (request.Handler, error) {

		return func(c *request.Context) {
			var reqID, transactionID, traceID string
			start := time.Now()
			tx := apm.TransactionFromContext(c.Request.Context())
			if tx != nil {
				// This request is being traced, grab its IDs to add to logs.
				traceContext := tx.TraceContext()
				transactionID = traceContext.Span.String()
				traceID = traceContext.Trace.String()
				reqID = transactionID
			} else {
				uuid, err := uuid.NewV4()
				if err != nil {
					id := request.IDResponseErrorsInternal
					logger.Errorw(request.MapResultIDToStatus[id].Keyword, "error", err)
					c.Result.SetWithError(id, err)
					c.Write()
					return
				}
				reqID = uuid.String()
			}

			reqLogger := logger.With(
				"request_id", reqID,
				"method", c.Request.Method,
				"URL", c.Request.URL,
				"content_length", c.Request.ContentLength,
				"remote_address", utility.RemoteAddr(c.Request),
				"user-agent", c.Request.Header.Get(headers.UserAgent))

			if traceID != "" {
				reqLogger = reqLogger.With(
					"trace.id", traceID,
					"transaction.id", transactionID,
				)
			}

			c.Logger = reqLogger
			h(c)
			reqLogger = reqLogger.With("event.duration", time.Since(start))

			if c.MultipleWriteAttempts() {
				reqLogger.Warn("multiple write attempts")
			}

			keyword := c.Result.Keyword
			if keyword == "" {
				keyword = "handled request"
			}

			keysAndValues := []interface{}{"response_code", c.Result.StatusCode}
			if c.Result.Err != nil {
				keysAndValues = append(keysAndValues, "error", c.Result.Err.Error())
			}
			if c.Result.Stacktrace != "" {
				keysAndValues = append(keysAndValues, "stacktrace", c.Result.Stacktrace)
			}

			if c.Result.Failure() {
				reqLogger.Errorw(keyword, keysAndValues...)
			} else {
				reqLogger.Infow(keyword, keysAndValues...)
			}

		}, nil
	}
}
