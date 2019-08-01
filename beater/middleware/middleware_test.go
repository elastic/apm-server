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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
)

func TestWithMiddleware(t *testing.T) {
	addKeyword := func(c *request.Context, s string) {
		c.Result.Keyword += s
	}
	h := func(c *request.Context) { addKeyword(c, "[h]") }
	m1 := func(h request.Handler) request.Handler {
		return func(c *request.Context) {
			addKeyword(c, "[m1.1]")
			h(c)
			addKeyword(c, "[m1.2]")
		}
	}
	m2 := func(h request.Handler) request.Handler {
		return func(c *request.Context) {
			addKeyword(c, "[m2.1]")
			h(c)
			addKeyword(c, "[m2.2]")
		}
	}
	c, _ := beatertest.DefaultContextWithResponseRecorder()
	WithMiddleware(h, m1, m2)(c)
	require.NotNil(t, c.Result.Keyword)
	assert.Equal(t, "[m1.1][m2.1][h][m2.2][m1.2]", c.Result.Keyword)
}
