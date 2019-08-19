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

package beatertest

import (
	"net/http"

	"github.com/elastic/apm-server/beater/request"
)

// Handler403 sets a 403 ID and status code to the context's response and calls Write()
func Handler403(c *request.Context) {
	c.Result.ID = request.IDResponseErrorsForbidden
	c.Result.StatusCode = http.StatusForbidden
	c.Write()
}

// Handler202 sets a 202 ID and status code to the context's response and calls Write()
func Handler202(c *request.Context) {
	c.Result.ID = request.IDResponseValidAccepted
	c.Result.StatusCode = http.StatusAccepted
	c.Write()
}

// HandlerPanic panics on request
func HandlerPanic(_ *request.Context) {
		panic("panic on Handle")
}

// HandlerIdle doesn't do anything but implement the request.Handler type
func HandlerIdle(c *request.Context) {}
