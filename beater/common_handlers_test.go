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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIncCounter(t *testing.T) {
	req, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	for i := 1; i <= 5; i++ {
		for _, res := range []serverResponse{acceptedResponse, okResponse, forbiddenResponse(errors.New("")), unauthorizedResponse,
			requestTooLargeResponse, rateLimitedResponse, methodNotAllowedResponse, tooManyConcurrentRequestsResponse,
			cannotValidateResponse(errors.New("")), cannotDecodeResponse(errors.New("")),
			fullQueueResponse(errors.New("")), serverShuttingDownResponse(errors.New(""))} {
			sendStatus(w, req, res)
			assert.Equal(t, int64(i), res.counter.Get())
		}
	}
	assert.Equal(t, int64(60), responseCounter.Get())
	assert.Equal(t, int64(50), responseErrors.Get())
}

type noopHandler struct {
	wg *sync.WaitGroup
}

func (h noopHandler) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {
	time.Sleep(time.Millisecond * 100)
	h.wg.Done()
}

func TestConcurrency(t *testing.T) {
	config := Config{ConcurrentRequests: 2, MaxRequestQueueTime: time.Second}
	var wg sync.WaitGroup
	h := concurrencyLimitHandler(&config, noopHandler{&wg})
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go h.ServeHTTP(nil, nil)
	}
	wg.Wait()
	// 3rd request must wait at least for one to complete
	assert.True(t, concurrentWait.Get() > 20, strconv.FormatInt(concurrentWait.Get(), 10))
}

func TestOPTIONS(t *testing.T) {
	config := defaultConfig("7.0.0")
	config.ConcurrentRequests = 1
	config.MaxRequestQueueTime = time.Second
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
		r := httptest.NewRequest("POST", "/", nil)
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
	req, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)
	w := httptest.NewRecorder()
	sendStatus(w, req, serverResponse{
		code:    http.StatusNonAuthoritativeInfo,
		counter: requestCounter,
		body:    "some body",
	})
	resp := w.Result()
	got, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "some body", string(got))
	assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
}

func TestOkBodyJson(t *testing.T) {
	req, err := http.NewRequest("POST", "_", nil)
	req.Header.Set("Accept", "application/json")
	assert.Nil(t, err)
	w := httptest.NewRecorder()
	sendStatus(w, req, serverResponse{
		code:    http.StatusNonAuthoritativeInfo,
		counter: requestCounter,
		body:    "some body",
	})
	resp := w.Result()
	got, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "{\"ok\":\"some body\"}", string(got))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestAccept(t *testing.T) {
	for idx, test := range []struct{ accept, expectedError, expectedContentType string }{
		{"application/json", "{\"error\":\"data validation error: error message\"}", "application/json"},
		{"*/*", "{\"error\":\"data validation error: error message\"}", "application/json"},
		{"text/html", "data validation error: error message", "text/plain; charset=utf-8"},
		{"", "data validation error: error message", "text/plain; charset=utf-8"},
	} {
		req, err := http.NewRequest("POST", "_", nil)
		assert.Nil(t, err)
		if test.accept != "" {
			req.Header.Set("Accept", test.accept)
		} else {
			delete(req.Header, "Accept")
		}
		w := httptest.NewRecorder()
		sendStatus(w, req, cannotValidateResponse(errors.New("error message")))
		resp := w.Result()
		body, _ := ioutil.ReadAll(resp.Body)
		assert.Equal(t, 400, w.Code)
		assert.Equal(t, test.expectedError, string(body), fmt.Sprintf("at index %d", idx))
		assert.Equal(t, test.expectedContentType, resp.Header.Get("Content-Type"), fmt.Sprintf("at index %d", idx))
	}
}

func TestIsAuthorized(t *testing.T) {
	reqAuth := func(auth string) *http.Request {
		req, err := http.NewRequest("POST", "_", nil)
		assert.Nil(t, err)
		req.Header.Add("Authorization", auth)
		return req
	}

	reqNoAuth, err := http.NewRequest("POST", "_", nil)
	assert.Nil(t, err)

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
