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

	"go.elastic.co/apm/v2"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/elastic/apm-server/internal/logs"
)

// LogMiddleware returns a middleware taking care of logging processing a request in the middleware and the request handler
func LogMiddleware() Middleware {
	return func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			c.Logger = loggerWithRequestContext(c)
			var err error
			if c.Logger, err = loggerWithTraceContext(c); err != nil {
				id := request.IDResponseErrorsInternal
				c.Logger.Error(request.MapResultIDToStatus[id].Keyword, logp.Error(err))
				c.Result.SetWithError(id, err)
				c.WriteResult()
				return
			}
			h(c)
			c.Logger = c.Logger.With("event.duration", time.Since(c.Timestamp))
			c.Logger = c.Logger.With("http.request.body.bytes", c.RequestBodyBytes())
			if c.MultipleWriteAttempts() {
				c.Logger.Warn("multiple write attempts")
			}
			keyword := c.Result.Keyword
			if keyword == "" {
				keyword = "handled request"
			}
			c.Logger = loggerWithResult(c)
			if c.Result.Failure() {
				c.Logger.Error(keyword)
				return
			}
			c.Logger.Info(keyword)
		}, nil
	}
}

func loggerWithRequestContext(c *request.Context) *logp.Logger {
	logger := logp.NewLogger(logs.Request).With(
		"url.original", c.Request.URL.String(),
		"http.request.method", c.Request.Method,
		"user_agent.original", c.Request.Header.Get(headers.UserAgent),
	)
	if c.SourceIP.IsValid() {
		logger = logger.With("source.address", c.SourceIP.String())
	}
	if c.ClientIP.IsValid() && c.ClientIP != c.SourceIP {
		logger = logger.With("client.ip", c.ClientIP.String())
	}
	return logger
}

func loggerWithTraceContext(c *request.Context) (*logp.Logger, error) {
	tx := apm.TransactionFromContext(c.Request.Context())
	if tx == nil {
		uuid, err := uuid.NewV4()
		if err != nil {
			return c.Logger, err
		}
		return c.Logger.With("http.request.id", uuid.String()), nil
	}
	// This request is being traced, grab its IDs to add to logs.
	traceContext := tx.TraceContext()
	transactionID := traceContext.Span.String()
	return c.Logger.With(
		"trace.id", traceContext.Trace.String(),
		"transaction.id", transactionID,
		"http.request.id", transactionID,
	), nil
}

func loggerWithResult(c *request.Context) *logp.Logger {
	logger := c.Logger.With(
		"http.response.status_code", c.Result.StatusCode)
	if c.Result.Err != nil {
		logger = logger.With("error.message", c.Result.Err.Error())
	}
	if c.Result.Stacktrace != "" {
		logger = logger.With("error.stack_trace", c.Result.Stacktrace)
	}
	return logger
}
