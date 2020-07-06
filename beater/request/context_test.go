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

package request

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/authorization"
	"github.com/elastic/apm-server/beater/headers"
)

func TestContext_Reset(t *testing.T) {
	w1 := httptest.NewRecorder()
	w1.WriteHeader(http.StatusServiceUnavailable)
	w2 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2 := httptest.NewRequest(http.MethodHead, "/new", nil)

	c := Context{
		Request: r1, w: w1,
		Logger: logp.NewLogger(""),
		Result: Result{
			StatusCode: http.StatusServiceUnavailable,
			Err:        errors.New("foo"),
			Stacktrace: "bar",
		},
	}
	c.Reset(w2, r2)

	// use reflection to ensure all fields of `context` are tested
	cType := reflect.TypeOf(c)
	cVal := reflect.ValueOf(c)
	for i := 0; i < cVal.NumField(); i++ {
		switch cType.Field(i).Name {
		case "Request":
			assert.Equal(t, r2, cVal.Field(i).Interface())
		case "Authorization":
			assert.Equal(t, &authorization.AllowAuth{}, cVal.Field(i).Interface())
		case "w":
			assert.Equal(t, w2, c.w)
		case "writeAttempts":
			assert.Equal(t, 0, c.writeAttempts)
		case "Result":
			assertResultIsEmpty(t, cVal.Field(i).Interface().(Result))
		case "RequestMetadata":
			assert.Equal(t, Metadata{}, cVal.Field(i).Interface().(Metadata))
		default:
			assert.Empty(t, cVal.Field(i).Interface(), cType.Field(i).Name)
		}
	}
}

func TestContext_Header(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set(headers.Etag, "abcd")
	w.Header().Set(headers.Bearer, "foo")
	c := Context{w: w}

	h := http.Header{headers.Etag: []string{"abcd"}, headers.Bearer: []string{"foo"}}
	assert.Equal(t, h, c.Header())
}

func TestContext_Write(t *testing.T) {

	t.Run("SecondWrite", func(t *testing.T) {
		c, w := mockContextAccept("*/*")
		c.Result = Result{Body: nil, StatusCode: http.StatusAccepted}
		c.Write()
		c.Result = Result{Body: nil, StatusCode: http.StatusBadRequest}
		c.Write()

		testHeaderXContentTypeOptions(t, c)
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("EmptyBody", func(t *testing.T) {
		c, w := mockContextAccept("*/*")
		c.Result = Result{Body: nil, StatusCode: http.StatusAccepted}
		c.Write()

		testHeaderXContentTypeOptions(t, c)
		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Empty(t, w.Body.String())
	})

	t.Run("WrapStringBodyInMap", func(t *testing.T) {
		c, w := mockContextAccept("")
		body := "bar"
		c.Result = Result{StatusCode: http.StatusBadRequest, Body: body}
		c.Write()

		testHeader(t, c, "text/plain; charset=utf-8")
		assert.Equal(t, `{"error":"bar"}`+"\n", w.Body.String())
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DoNotWrapStringBody", func(t *testing.T) {
		c, w := mockContextAccept("text/html")
		body := "bar"
		c.Result = Result{StatusCode: http.StatusOK, Body: body}
		c.Write()

		testHeader(t, c, "text/plain; charset=utf-8")
		assert.Equal(t, `bar`+"\n", w.Body.String())
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DoNotWrapOtherBodyInMap", func(t *testing.T) {
		c, w := mockContextAccept("application/text")
		body := map[string]interface{}{"xyz": "bar"}
		c.Result = Result{StatusCode: http.StatusBadRequest, Body: body}
		c.Write()

		testHeader(t, c, "text/plain; charset=utf-8")
		assert.Equal(t, `{"xyz":"bar"}`+"\n", w.Body.String())
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Accept", func(t *testing.T) {
		for name, tc := range map[string]struct {
			acceptHeader                 string
			body                         interface{}
			expectedHeader, expectedBody string
		}{
			"application/json": {
				acceptHeader:   "application/json",
				body:           map[string]interface{}{"xyz": "bar"},
				expectedHeader: "application/json",
				expectedBody: `{
  "xyz": "bar"
}
`,
			},
			"*/*": {
				acceptHeader:   "*/*",
				body:           map[string]interface{}{"xyz": "bar"},
				expectedHeader: "application/json",
				expectedBody: `{
  "xyz": "bar"
}
`,
			},
			"jsonBody": {
				acceptHeader:   "application/text",
				body:           map[string]interface{}{"xyz": "bar"},
				expectedHeader: "text/plain; charset=utf-8",
				expectedBody:   `{"xyz":"bar"}` + "\n",
			},
			"application/text": {
				acceptHeader:   "application/text",
				body:           "foo",
				expectedHeader: "text/plain; charset=utf-8",
				expectedBody:   `foo` + "\n",
			},
			"empty": {
				acceptHeader:   "",
				body:           "foo",
				expectedHeader: "text/plain; charset=utf-8",
				expectedBody:   `foo` + "\n",
			},
		} {
			t.Run(name, func(t *testing.T) {
				c, w := mockContextAccept(tc.acceptHeader)
				c.Result = Result{StatusCode: http.StatusNotModified, Body: tc.body}
				c.Write()

				testHeader(t, c, tc.expectedHeader)
				assert.Equal(t, tc.expectedBody, w.Body.String())
				assert.Equal(t, http.StatusNotModified, w.Code)
			})
		}
	})
}

func testHeaderXContentTypeOptions(t *testing.T, c *Context) {
	assert.Equal(t, "nosniff", c.w.Header().Get(headers.XContentTypeOptions))
}

func testHeader(t *testing.T, c *Context, expected string) {
	assert.Equal(t, expected, c.w.Header().Get(headers.ContentType))
	testHeaderXContentTypeOptions(t, c)
}

func mockContextAccept(accept string) (*Context, *httptest.ResponseRecorder) {
	c := &Context{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodHead, "/", nil)
	r.Header.Set(headers.Accept, accept)
	c.Reset(w, r)
	return c, w

}
