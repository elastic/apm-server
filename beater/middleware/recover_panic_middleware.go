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
	"runtime/debug"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/beater/request"
)

const keywordPanic = "panic handling request"

// RecoverPanicMiddleware returns a middleware ensuring that the Server recovers from panics,
// while trying to write an according response.
func RecoverPanicMiddleware() Middleware {
	return func(h request.Handler) request.Handler {
		return func(c *request.Context) {

			defer func() {
				if r := recover(); r != nil {

					// recover again in case setting the context's result or writing the response itself
					// is throwing the panic
					defer func() {
						recover()
					}()

					id := request.IDResponseErrorsInternal
					status := request.MapResultIDToStatus[id]

					// set the context's result and write response
					var ok bool
					var err error
					if err, ok = r.(error); !ok {
						err = errors.Wrap(err, status.Keyword)
					}
					c.Result.Set(id, status.Code, status.Keyword, keywordPanic, err)
					c.Result.Stacktrace = string(debug.Stack())

					c.Write()
				}
			}()
			h(c)
		}
	}
}
