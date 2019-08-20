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
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/elastic/apm-server/beater/request"
)

// DefaultContextWithResponseRecorder returns a context and response recorder for testing purposes
// It is set to send a GET request to the root path.
func DefaultContextWithResponseRecorder() (*request.Context, *httptest.ResponseRecorder) {
	return ContextWithResponseRecorder(http.MethodGet, "/")
}

// ContextWithResponseRecorder returns a custom context and response recorder for testing purposes
// It is set to use the passed in request method and path
func ContextWithResponseRecorder(m string, target string) (*request.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c := &request.Context{}
	c.Reset(w, httptest.NewRequest(m, target, nil))
	return c, w
}

// WriterPanicOnce implements the http.ResponseWriter interface
// It panics once when any method is called.
type WriterPanicOnce struct {
	StatusCode int
	Body       bytes.Buffer
	panicked   bool
}

// Header panics if it is the first call to the struct, otherwise returns empty Header
func (w *WriterPanicOnce) Header() http.Header {
	if !w.panicked {
		w.panicked = true
		panic(errors.New("panic on Header"))
	}
	return http.Header{}
}

// Write panics if it is the first call to the struct, otherwise it writes the given bytes to the body
func (w *WriterPanicOnce) Write(b []byte) (int, error) {
	if !w.panicked {
		w.panicked = true
		panic(errors.New("panic on Write"))
	}
	return w.Body.Write(b)
}

// WriteHeader panics if it is the first call to the struct, otherwise it writes the given status code
func (w *WriterPanicOnce) WriteHeader(statusCode int) {
	if !w.panicked {
		w.panicked = true
		panic(errors.New("panic on WriteHeader"))
	}
	w.StatusCode = statusCode
}
