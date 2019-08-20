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
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/beater/beatertest"
	"github.com/elastic/apm-server/beater/request"
)

func TestPanicHandler(t *testing.T) {

	t.Run("NoPanic", func(t *testing.T) {
		c, w := beatertest.DefaultContextWithResponseRecorder()
		RecoverPanicMiddleware()(beatertest.Handler403)(c)

		// response assertions
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "", w.Body.String())
		// result assertions e.g. for logging
		assert.Nil(t, c.Result.Err)
		assert.Empty(t, c.Result.Stacktrace)
	})

	t.Run("HandlePanic", func(t *testing.T) {
		h := func(c *request.Context) { panic(errors.New("panic xyz")) }
		c, w := beatertest.DefaultContextWithResponseRecorder()
		RecoverPanicMiddleware()(h)(c)

		// response assertions
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, beatertest.ResultErrWrap(keywordPanic), w.Body.String())

		// result assertions e.g. for logging
		assert.NotNil(t, c.Result.Err)
		assert.NotEmpty(t, c.Result.Stacktrace)
		assert.Equal(t, keywordPanic, c.Result.Body)
	})

	t.Run("SecondPanic", func(t *testing.T) {
		w := &writerPanic{}
		c := &request.Context{}
		c.Reset(w, httptest.NewRequest(http.MethodGet, "/", nil))
		RecoverPanicMiddleware()(beatertest.Handler202)(c)

		// response assertions: cannot even write to writer, as it panics
		assert.Equal(t, 0, w.StatusCode)
		assert.Empty(t, "", w.Body.String())

		// result assertions e.g. for logging
		assert.NotNil(t, c.Result.Err)
		assert.NotEmpty(t, c.Result.Stacktrace)
		assert.Equal(t, keywordPanic, c.Result.Body)
	})
}

type writerPanic struct {
	StatusCode int
	Body       bytes.Buffer
}

func (w *writerPanic) Header() http.Header {
	panic(errors.New("panic returning header"))
}
func (w *writerPanic) Write(b []byte) (int, error) {
	panic(errors.New("panic writing"))
}
func (w *writerPanic) WriteHeader(statusCode int) {
	panic(errors.New("panic writing header"))
}
