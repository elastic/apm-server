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
	"net/http"
	"testing"

	"github.com/elastic/apm-server/internal/beater/request"
	"github.com/stretchr/testify/assert"
)

func TestAsyncMiddleware(t *testing.T) {
	test := func(r *http.Request) *request.Context {
		m := AsyncMiddleware()
		h := request.Handler(func(c *request.Context) {})
		handle, err := m(h)
		assert.NoError(t, err)

		c := request.NewContext()
		c.Request = r

		handle(c)
		return c
	}
	t.Run("true", func(t *testing.T) {
		r, err := http.NewRequest("POST", "/intake/v2/events?async=true", nil)
		assert.NoError(t, err)
		c := test(r)
		assert.True(t, c.Async)
	})
	t.Run("false", func(t *testing.T) {
		r, err := http.NewRequest("POST", "/intake/v2/events?async=false", nil)
		assert.NoError(t, err)
		c := test(r)
		assert.False(t, c.Async)
	})
	t.Run("missing", func(t *testing.T) {
		r, err := http.NewRequest("POST", "/intake/v2/events", nil)
		assert.NoError(t, err)
		c := test(r)
		assert.False(t, c.Async)
	})
}
