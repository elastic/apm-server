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
			args, err := requestArgs(c, logger.ECSEnabled())
			if err != nil {
				id := request.IDResponseErrorsInternal
				logger.Errorw(request.MapResultIDToStatus[id].Keyword, "error", err)
				c.Result.SetWithError(id, err)
				c.Write()
				return
			}
			c.Logger = logger.With(args...)
			h(c)

			if c.MultipleWriteAttempts() {
				c.Logger.Warn("multiple write attempts")
			}
			keyword := c.Result.Keyword
			if keyword == "" {
				keyword = "handled request"
			}
			args = resultArgs(c, logger.ECSEnabled())
			if c.Result.Failure() {
				c.Logger.Errorw(keyword, args...)
				return
			}
			c.Logger.Infow(keyword, args...)
		}, nil
	}
}

func requestArgs(c *request.Context, ecsEnabled bool) ([]interface{}, error) {
	var reqID, transactionID, traceID string
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
			return nil, err
		}
		reqID = uuid.String()
	}

	args := []interface{}{
		"http", map[string]interface{}{
			"request": map[string]interface{}{
				"id":     reqID, //not defined in ECS but fits here best
				"method": c.Request.Method,
				"body":   map[string]interface{}{"bytes": c.Request.ContentLength}}},
		"source", map[string]interface{}{"address": utility.RemoteAddr(c.Request)},
		"user_agent", map[string]interface{}{"original": c.Request.Header.Get(headers.UserAgent)},
	}
	if traceID != "" {
		args = append(args,
			"trace", map[string]interface{}{"id": traceID},
			"transaction", map[string]interface{}{"id": transactionID})
	}
	// avoid conflicts on existing log keys
	if ecsEnabled {
		return append(args, "url", map[string]string{"original": c.Request.URL.String()}), nil
	}
	return append(args, "URL", c.Request.URL), nil
}

func resultArgs(c *request.Context, ecsEnabled bool) []interface{} {
	args := []interface{}{
		// http key will be duplicated at this point
		"http", map[string]interface{}{
			"response": map[string]interface{}{
				"status_code": c.Result.StatusCode}}}
	if c.Result.Err == nil && c.Result.Stacktrace == "" {
		return args
	}

	if ecsEnabled {
		err := map[string]interface{}{}
		if c.Result.Err != nil {
			err["message"] = c.Result.Err.Error()
		}
		if c.Result.Stacktrace != "" {
			err["stacktrace"] = c.Result.Stacktrace
		}
		return append(args, "error", err)
	}
	if c.Result.Err != nil {
		args = append(args, "error", c.Result.Err)
	}
	if c.Result.Stacktrace != "" {
		args = append(args, "stacktrace", c.Result.Stacktrace)
	}
	return args
}
