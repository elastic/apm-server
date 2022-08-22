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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/internal/beater/request"
)

func TestWithMiddleware(t *testing.T) {
	addKeyword := func(c *request.Context, s string) {
		c.Result.Keyword += s
	}
	h := func(c *request.Context) { addKeyword(c, "[h]") }
	m1 := func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			addKeyword(c, "[m1.1]")
			h(c)
			addKeyword(c, "[m1.2]")
		}, nil
	}
	m2 := func(h request.Handler) (request.Handler, error) {
		return func(c *request.Context) {
			addKeyword(c, "[m2.1]")
			h(c)
			addKeyword(c, "[m2.2]")
		}, nil
	}
	c, _ := DefaultContextWithResponseRecorder()
	h, e := Wrap(h, m1, m2)
	require.NoError(t, e)
	h(c)
	require.NotNil(t, c.Result.Keyword)
	assert.Equal(t, "[m1.1][m2.1][h][m2.2][m1.2]", c.Result.Keyword)
}

func Apply(m Middleware, h request.Handler) request.Handler {
	r, _ := m(h)
	return r
}

// ResultErrWrap wraps given input into the expected result error string
func ResultErrWrap(s string) string {
	return fmt.Sprintf("{\"error\":\"%+v\"}\n", s)
}

// Handler403 sets a full 403 result and calls WriteResult()
func Handler403(c *request.Context) {
	c.Result.Set(request.IDResponseErrorsForbidden,
		http.StatusForbidden,
		request.MapResultIDToStatus[request.IDResponseErrorsForbidden].Keyword,
		nil,
		nil)
	c.WriteResult()
}

// Handler202 sets a 202 ID, status code and keyword to the context's response and calls WriteResult()
func Handler202(c *request.Context) {
	c.Result.ID = request.IDResponseValidAccepted
	c.Result.StatusCode = http.StatusAccepted
	c.Result.Keyword = request.MapResultIDToStatus[request.IDResponseValidAccepted].Keyword
	c.WriteResult()
}

// HandlerPanic panics on request
func HandlerPanic(_ *request.Context) {
	panic(errors.New("panic on Handle"))
}

// HandlerIdle doesn't do anything but implement the request.Handler type
func HandlerIdle(c *request.Context) {}

// DefaultContextWithResponseRecorder returns a context and response recorder for testing purposes
// It is set to send a GET request to the root path.
func DefaultContextWithResponseRecorder() (*request.Context, *httptest.ResponseRecorder) {
	c := request.NewContext()
	rec := httptest.NewRecorder()
	c.Reset(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return c, rec
}
