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
	"net/http"

	"github.com/gofrs/uuid"

	"github.com/elastic/beats/libbeat/logp"

	"github.com/elastic/apm-server/beater/headers"
	"github.com/elastic/apm-server/beater/request"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/utility"
)

func logHandler(h Handler) Handler {
	logger := logp.NewLogger(logs.Request)

	return func(c *request.Context) {
		requestCounter.Inc()
		reqID, err := uuid.NewV4()
		if err != nil {
			sendStatus(c, internalErrorResponse(err))
		}

		reqLogger := logger.With(
			"request_id", reqID,
			"method", c.Request.Method,
			"URL", c.Request.URL,
			"content_length", c.Request.ContentLength,
			"remote_address", utility.RemoteAddr(c.Request),
			"user-agent", c.Request.Header.Get(headers.UserAgent))

		h(c)

		keysAndValues := []interface{}{"response_code", c.StatusCode}
		if c.StatusCode >= http.StatusBadRequest {
			keysAndValues = append(keysAndValues, "error", c.Err)
			if c.Stacktrace != "" {
				keysAndValues = append(keysAndValues, "stacktrace", c.Stacktrace)
			}
			reqLogger.Errorw("error handling request", keysAndValues...)
			return
		}

		reqLogger.Infow("handled request", keysAndValues...)
	}
}
