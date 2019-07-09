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

package beater

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncCounter(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "_", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	responseCounter.Set(0)
	responseErrors.Set(0)
	for _, res := range []serverResponse{acceptedResponse, okResponse, forbiddenResponse(errors.New("")), unauthorizedResponse,
		requestTooLargeResponse, rateLimitedResponse, methodNotAllowedResponse,
		cannotValidateResponse(errors.New("")), cannotDecodeResponse(errors.New("")),
		fullQueueResponse(errors.New("")), serverShuttingDownResponse(errors.New(""))} {
		res.counter.Set(0)
		for i := 1; i <= 5; i++ {
			sendStatus(w, req, res)
			assert.Equal(t, int64(i), res.counter.Get(), string(res.code))
		}
	}
	assert.Equal(t, int64(55), responseCounter.Get())
	assert.Equal(t, int64(45), responseErrors.Get())
}

func TestOPTIONS(t *testing.T) {
	config := defaultConfig("7.0.0")
	enabled := true
	config.RumConfig.Enabled = &enabled

	requestTaken := make(chan struct{}, 1)
	done := make(chan struct{}, 1)

	h := rumHandler(config, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestTaken <- struct{}{}
		<-done
	}))

	// use this to block the single allowed concurrent requests
	go func() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		h.ServeHTTP(w, r)
	}()

	<-requestTaken

	// send a new request which should be allowed through
	w := httptest.NewRecorder()
	r := httptest.NewRequest("OPTIONS", "/", nil)
	h.ServeHTTP(w, r)
	assert.Equal(t, 200, w.Code, w.Body.String())
	done <- struct{}{}
}

func TestOkBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "_", nil)
	assert.Nil(t, err)
	w := httptest.NewRecorder()
	sendStatus(w, req, serverResponse{
		code:    http.StatusNonAuthoritativeInfo,
		counter: requestCounter,
		body:    map[string]interface{}{"some": "body"},
	})
	rsp := w.Result()
	got := body(t, rsp)
	assert.Equal(t, "{\"some\":\"body\"}\n", string(got))
	assert.Equal(t, "text/plain; charset=utf-8", rsp.Header.Get("Content-Type"))
}

func TestOkBodyJson(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "_", nil)
	req.Header.Set("Accept", "application/json")
	assert.Nil(t, err)
	w := httptest.NewRecorder()
	sendStatus(w, req, serverResponse{
		code:    http.StatusNonAuthoritativeInfo,
		counter: requestCounter,
		body:    map[string]interface{}{"version": "1.0"},
	})
	rsp := w.Result()
	got := body(t, rsp)
	assert.Equal(t,
		`{
  "version": "1.0"
}
`, string(got))
	assert.Equal(t, "application/json", rsp.Header.Get("Content-Type"))
}

func TestAccept(t *testing.T) {
	expectedErrorJson :=
		`{
  "error": "data validation error: error message"
}
`
	expectedErrorText := "{\"error\":\"data validation error: error message\"}\n"

	for idx, test := range []struct{ accept, expectedError, expectedContentType string }{
		{"application/json", expectedErrorJson, "application/json"},
		{"*/*", expectedErrorJson, "application/json"},
		{"text/html", expectedErrorText, "text/plain; charset=utf-8"},
		{"", expectedErrorText, "text/plain; charset=utf-8"},
	} {
		req, err := http.NewRequest(http.MethodPost, "_", nil)
		require.NoError(t, err)
		if test.accept != "" {
			req.Header.Set("Accept", test.accept)
		} else {
			delete(req.Header, "Accept")
		}
		w := httptest.NewRecorder()
		sendStatus(w, req, cannotValidateResponse(errors.New("error message")))
		rsp := w.Result()
		got := body(t, rsp)
		assert.Equal(t, 400, w.Code)
		assert.Equal(t, test.expectedError, got, fmt.Sprintf("at index %d", idx))
		assert.Equal(t, test.expectedContentType, rsp.Header.Get("Content-Type"), fmt.Sprintf("at index %d", idx))
	}
}

func TestIsAuthorized(t *testing.T) {
	reqAuth := func(auth string) *http.Request {
		req, err := http.NewRequest(http.MethodPost, "_", nil)
		assert.Nil(t, err)
		req.Header.Add("Authorization", auth)
		return req
	}

	reqNoAuth, err := http.NewRequest(http.MethodPost, "_", nil)
	require.NoError(t, err)

	// Successes
	assert.True(t, isAuthorized(reqNoAuth, ""))
	assert.True(t, isAuthorized(reqAuth("foo"), ""))
	assert.True(t, isAuthorized(reqAuth("Bearer foo"), "foo"))

	// Failures
	assert.False(t, isAuthorized(reqNoAuth, "foo"))
	assert.False(t, isAuthorized(reqAuth("Bearer bar"), "foo"))
	assert.False(t, isAuthorized(reqAuth("Bearer foo extra"), "foo"))
	assert.False(t, isAuthorized(reqAuth("foo"), "foo"))
}
