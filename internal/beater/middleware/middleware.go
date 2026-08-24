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
	"slices"

	"github.com/elastic/apm-server/internal/beater/request"
)

// Middleware wraps a request.Handler
type Middleware func(request.Handler) (request.Handler, error)

// Wrap wraps a request.Handler into given middleware functions,
// maintaining order from the last to the first middleware
func Wrap(h request.Handler, m ...Middleware) (request.Handler, error) {
	for _, v := range slices.Backward(m) {
		var e error
		h, e = v(h)
		if e != nil {
			return nil, e
		}
	}
	return h, nil
}
