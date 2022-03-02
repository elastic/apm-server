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
	"bytes"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/headers"
)

func TestContext_Reset(t *testing.T) {
	w1 := httptest.NewRecorder()
	w1.WriteHeader(http.StatusServiceUnavailable)
	w2 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodGet, "/", nil)
	r1.RemoteAddr = "10.1.2.3:4321"
	r1.Header.Set("User-Agent", "ua1")
	r2 := httptest.NewRequest(http.MethodHead, "/new", nil)
	r2.RemoteAddr = "10.1.2.3:1234"
	r2.Header.Set("User-Agent", "ua2")
	r2.Header.Set("X-Forwarded-For", "192.168.0.1")

	var multipartBuf bytes.Buffer
	multipartWriter := multipart.NewWriter(&multipartBuf)
	fw, err := multipartWriter.CreateFormFile("a_file", "filename.txt")
	require.NoError(t, err)
	fw.Write([]byte("abc"))
	err = multipartWriter.Close()
	require.NoError(t, err)

	multipartReader := multipart.NewReader(&multipartBuf, multipartWriter.Boundary())
	form, err := multipartReader.ReadForm(0) // always write to /tmp
	require.NoError(t, err)
	r1.MultipartForm = form

	// Check that a temp file was written.
	require.Len(t, form.File["a_file"], 1)
	formFile, err := form.File["a_file"][0].Open()
	require.NoError(t, err)
	formTempFile := formFile.(*os.File)
	formTempFilename := formTempFile.Name()
	formFile.Close()

	before := time.Now()
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

	// Resetting the context should have removed r1's temporary form file.
	_, err = os.Stat(formTempFilename)
	require.True(t, os.IsNotExist(err))

	// use reflection to ensure all fields of `context` are tested
	cType := reflect.TypeOf(c)
	cVal := reflect.ValueOf(c)
	for i := 0; i < cVal.NumField(); i++ {
		switch cType.Field(i).Name {
		case "Request":
			assert.Equal(t, r2, cVal.Field(i).Interface())
		case "Authentication":
			assert.Equal(t, auth.AuthenticationDetails{}, cVal.Field(i).Interface())
		case "w":
			assert.Equal(t, w2, c.w)
		case "writeAttempts":
			assert.Equal(t, 0, c.writeAttempts)
		case "Result":
			assertResultIsEmpty(t, cVal.Field(i).Interface().(Result))
		case "SourceIP":
			assert.Equal(t, net.ParseIP("10.1.2.3"), cVal.Field(i).Interface())
		case "SourcePort":
			assert.Equal(t, 1234, cVal.Field(i).Interface())
		case "ClientIP":
			assert.Equal(t, net.ParseIP("10.1.2.3"), cVal.Field(i).Interface())
		case "ClientPort":
			assert.Equal(t, 1234, cVal.Field(i).Interface())
		case "SourceNATIP":
			assert.Equal(t, net.ParseIP("192.168.0.1"), cVal.Field(i).Interface())
		case "SourceNATPort":
			assert.Equal(t, 0, cVal.Field(i).Interface())
		case "UserAgent":
			assert.Equal(t, "ua2", cVal.Field(i).Interface())
		case "Timestamp":
			timestamp := cVal.Field(i).Interface().(time.Time)
			assert.False(t, timestamp.Before(before))
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
